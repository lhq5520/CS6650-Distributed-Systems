"""
Unit tests for Leader-Follower database consistency.

These tests run against a live Leader-Follower cluster (docker-compose-leader.yml).
Default config: W=5, R=1 (all nodes must confirm write, read from leader only).

Port mapping:
  - Leader:    localhost:8080
  - Follower1: localhost:8081
  - Follower2: localhost:8082
  - Follower3: localhost:8083
  - Follower4: localhost:8084
"""

import requests
import time
import threading
import pytest

LEADER = "http://localhost:8080"
FOLLOWERS = [
    "http://localhost:8081",
    "http://localhost:8082",
    "http://localhost:8083",
    "http://localhost:8084",
]


def set_key(url, key, value):
    """Send a set request and return the response."""
    resp = requests.post(f"{url}/set", json={"key": key, "value": value})
    return resp


def get_key(url, key):
    """Send a get request and return the response."""
    resp = requests.get(f"{url}/get", params={"key": key})
    return resp


def local_read(url, key):
    """Send a local_read request to peek at a node's local data."""
    resp = requests.get(f"{url}/local_read", params={"key": key})
    return resp


# =============================================
# Test 1: After write completes, Leader is consistent
# =============================================
def test_leader_read_after_write():
    """
    Write to Leader, then read from Leader.
    With any W configuration, the Leader always has the latest data
    because it stores locally before replicating.
    """
    resp = set_key(LEADER, "test1", "hello")
    assert resp.status_code == 201

    resp = get_key(LEADER, "test1")
    assert resp.status_code == 200
    data = resp.json()
    assert data["value"] == "hello"
    assert data["version"] >= 1
    print(f"  Leader read after write: value={data['value']}, version={data['version']}")


# =============================================
# Test 2: After write completes (W=5), Followers are consistent
# =============================================
def test_follower_consistent_after_write_w5():
    """
    With W=5, the Leader waits for ALL followers to confirm.
    So after set() returns, every follower must have the data.
    """
    resp = set_key(LEADER, "test2", "world")
    assert resp.status_code == 201

    for i, follower in enumerate(FOLLOWERS):
        resp = local_read(follower, "test2")
        assert resp.status_code == 200
        data = resp.json()
        assert data["value"] == "world"
        print(f"  Follower{i+1} local_read after W=5 write: value={data['value']}, version={data['version']}")


# =============================================
# Test 3: Expose inconsistency during replication using local_read
# =============================================
def test_inconsistency_during_replication():
    """
    This test tries to catch the inconsistency window.
    We send a write to the Leader and IMMEDIATELY read local_read
    from followers. During the replication delay (Leader sleeps 200ms
    per follower, follower sleeps 100ms), some followers may not have
    the new value yet.

    We use W=1 config here conceptually, but even with W=5 we can
    peek with local_read during the replication process by reading
    in parallel with the write.
    """
    key = "test3_inconsistency"
    # First, write an initial value so all nodes have version 1
    resp = set_key(LEADER, key, "version_1")
    assert resp.status_code == 201
    time.sleep(1)  # wait for full replication

    # Now send a NEW write in a background thread.
    # While it's replicating, immediately local_read from followers.
    inconsistency_found = False
    write_done = threading.Event()

    def do_write():
        set_key(LEADER, key, "version_2")
        write_done.set()

    thread = threading.Thread(target=do_write)
    thread.start()

    # Immediately try to read from all followers multiple times
    # The replication takes time (200ms sleep per follower + 100ms on follower)
    # so we should catch some followers still on "version_1"
    attempts = 0
    for _ in range(20):
        for follower in FOLLOWERS:
            try:
                resp = local_read(follower, key)
                if resp.status_code == 200:
                    data = resp.json()
                    if data["value"] == "version_1":
                        inconsistency_found = True
                        attempts += 1
            except Exception:
                pass
        time.sleep(0.01)  # 10ms between rounds

    thread.join()

    # After write completes, verify all followers are now consistent
    for follower in FOLLOWERS:
        resp = local_read(follower, key)
        assert resp.status_code == 200
        data = resp.json()
        assert data["value"] == "version_2"

    print(f"  Inconsistency detected: {inconsistency_found} (stale reads: {attempts})")
    print(f"  After write completed: all followers consistent with 'version_2'")
    # We don't assert inconsistency_found because it's timing-dependent,
    # but at high load it should happen.


if __name__ == "__main__":
    pytest.main([__file__, "-v", "-s"])
