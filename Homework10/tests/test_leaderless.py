"""
Unit tests for Leaderless database consistency.

These tests run against a live Leaderless cluster (docker-compose-leaderless.yml).
Config: W=N (all nodes), R=1 (read local only).

Port mapping:
  - Node1: localhost:8090
  - Node2: localhost:8091
  - Node3: localhost:8092
  - Node4: localhost:8093
  - Node5: localhost:8094
"""

import requests
import time
import threading
import random
import pytest

NODES = [
    "http://localhost:8090",
    "http://localhost:8091",
    "http://localhost:8092",
    "http://localhost:8093",
    "http://localhost:8094",
]


def set_key(url, key, value):
    resp = requests.post(f"{url}/set", json={"key": key, "value": value})
    return resp


def get_key(url, key):
    resp = requests.get(f"{url}/get", params={"key": key})
    return resp


# =============================================
# Test 1: After write completes, Coordinator is consistent
# =============================================
def test_coordinator_consistent_after_write():
    """
    Write to a random node (it becomes the Write Coordinator).
    After the write returns (W=N, all nodes confirmed),
    reading from the Coordinator should return the correct value.
    """
    coordinator = random.choice(NODES)
    resp = set_key(coordinator, "ltest1", "hello_leaderless")
    assert resp.status_code == 201

    resp = get_key(coordinator, "ltest1")
    assert resp.status_code == 200
    data = resp.json()
    assert data["value"] == "hello_leaderless"
    print(f"  Coordinator ({coordinator}) read after write: value={data['value']}, version={data['version']}")


# =============================================
# Test 2: After write completes (W=N), ALL nodes are consistent
# =============================================
def test_all_nodes_consistent_after_write():
    """
    With W=N, the Coordinator waits for ALL nodes to confirm.
    So after set() returns, every node must have the data.
    """
    coordinator = NODES[0]
    resp = set_key(coordinator, "ltest2", "all_synced")
    assert resp.status_code == 201

    for i, node in enumerate(NODES):
        resp = get_key(node, "ltest2")
        assert resp.status_code == 200
        data = resp.json()
        assert data["value"] == "all_synced"
        print(f"  Node{i+1} read after W=N write: value={data['value']}, version={data['version']}")


# =============================================
# Test 3: Expose inconsistency window during write propagation
# =============================================
def test_inconsistency_window():
    """
    This is the key test for Leaderless:
    1. Write an initial value to all nodes.
    2. Start a NEW write in the background (via one node as Coordinator).
    3. While the Coordinator is propagating, read from OTHER nodes.
    4. Some nodes may still have the old value -> inconsistency!
    5. After write completes, all nodes should be consistent.
    """
    key = "ltest3_inconsistency"
    coordinator = NODES[0]
    other_nodes = NODES[1:]

    # Step 1: Write initial value, wait for full propagation
    resp = set_key(coordinator, key, "old_value")
    assert resp.status_code == 201
    time.sleep(1)

    # Step 2 & 3: Write new value in background, immediately read others
    inconsistency_found = False
    stale_count = 0

    def do_write():
        set_key(coordinator, key, "new_value")

    thread = threading.Thread(target=do_write)
    thread.start()

    # Immediately read from other nodes while write is propagating
    # Coordinator sleeps 200ms per peer + peer sleeps 100ms,
    # so the window is several hundred milliseconds
    for _ in range(30):
        for node in other_nodes:
            try:
                resp = get_key(node, key)
                if resp.status_code == 200:
                    data = resp.json()
                    if data["value"] == "old_value":
                        inconsistency_found = True
                        stale_count += 1
            except Exception:
                pass
        time.sleep(0.01)  # 10ms between rounds

    thread.join()

    # Step 4: After write completes, verify ALL nodes are consistent
    for i, node in enumerate(NODES):
        resp = get_key(node, key)
        assert resp.status_code == 200
        data = resp.json()
        assert data["value"] == "new_value"
        print(f"  Node{i+1} after write: value={data['value']}, version={data['version']}")

    print(f"  Inconsistency detected during write: {inconsistency_found} (stale reads: {stale_count})")


# =============================================
# Test 4: Different coordinators can write to the same key
# =============================================
def test_different_coordinators():
    """
    In a leaderless system, any node can be the Write Coordinator.
    Write the same key from different nodes sequentially.
    The version should increment correctly.
    """
    key = "ltest4_multi_coord"

    for i, node in enumerate(NODES):
        resp = set_key(node, key, f"value_from_node{i+1}")
        assert resp.status_code == 201
        data = resp.json()
        print(f"  Write via Node{i+1}: version={data['version']}")
        time.sleep(0.5)  # wait for propagation

    # Final read from a random node should return the last write
    resp = get_key(random.choice(NODES), key)
    assert resp.status_code == 200
    data = resp.json()
    assert data["value"] == "value_from_node5"
    print(f"  Final read: value={data['value']}, version={data['version']}")


if __name__ == "__main__":
    pytest.main([__file__, "-v", "-s"])
