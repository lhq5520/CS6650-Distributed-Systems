"""
Load test client for distributed KV databases.

Tests 4 configurations x 4 read/write ratios.
Records latency per request and counts stale reads.

Usage:
  python load_test.py

The script will prompt you to start the right docker-compose for each config.
Results are saved to JSON files and graphs are generated automatically.
"""

import requests
import time
import random
import json
import os
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed

# ============================================================
# Configuration
# ============================================================

NUM_REQUESTS = 200        # total requests per test run
NUM_KEYS = 10             # small key pool for "local-in-time" data
NUM_THREADS = 10          # concurrent clients
KEY_PREFIX = "loadtest_"

WRITE_RATIOS = [0.01, 0.10, 0.50, 0.90]  # 1%, 10%, 50%, 90% writes

CONFIGS = {
    "leader_W5_R1": {
        "compose_file": "docker-compose-leader.yml",
        "env": {"W": "5", "R": "1"},
        "write_url": "http://localhost:8080",   # writes go to Leader
        "read_urls": ["http://localhost:8080"],  # R=1, read from Leader only
        "follower_urls": [
            "http://localhost:8081",
            "http://localhost:8082",
            "http://localhost:8083",
            "http://localhost:8084",
        ],
    },
    "leader_W1_R5": {
        "compose_file": "docker-compose-leader.yml",
        "env": {"W": "1", "R": "5"},
        "write_url": "http://localhost:8080",
        "read_urls": ["http://localhost:8080"],  # R=5 handled server-side
        "follower_urls": [
            "http://localhost:8081",
            "http://localhost:8082",
            "http://localhost:8083",
            "http://localhost:8084",
        ],
    },
    "leader_W3_R3": {
        "compose_file": "docker-compose-leader.yml",
        "env": {"W": "3", "R": "3"},
        "write_url": "http://localhost:8080",
        "read_urls": ["http://localhost:8080"],  # R=3 handled server-side
        "follower_urls": [
            "http://localhost:8081",
            "http://localhost:8082",
            "http://localhost:8083",
            "http://localhost:8084",
        ],
    },
    "leaderless_WN_R1": {
        "compose_file": "docker-compose-leaderless.yml",
        "env": {},
        "write_url": None,  # any node can write
        "read_urls": [
            "http://localhost:8090",
            "http://localhost:8091",
            "http://localhost:8092",
            "http://localhost:8093",
            "http://localhost:8094",
        ],
        "follower_urls": [],
    },
}

# ============================================================
# Client-side version tracking (to detect stale reads)
# ============================================================

class VersionTracker:
    """
    Tracks the latest known version for each key.
    When a write returns version=N, we record it.
    When a read returns version<N, that's a stale read.
    """
    def __init__(self):
        self.versions = {}  # key -> latest known version

    def update(self, key, version):
        if key not in self.versions or version > self.versions[key]:
            self.versions[key] = version

    def is_stale(self, key, version):
        if key not in self.versions:
            return False
        return version < self.versions[key]


# ============================================================
# Load test runner
# ============================================================

def do_write(url, key, value):
    """Perform a write and return (latency_ms, version)."""
    start = time.time()
    try:
        resp = requests.post(f"{url}/set", json={"key": key, "value": value}, timeout=10)
        latency = (time.time() - start) * 1000
        if resp.status_code == 201:
            data = resp.json()
            return latency, data.get("version", 0)
        return latency, -1
    except Exception:
        return (time.time() - start) * 1000, -1


def do_read(url, key):
    """Perform a read and return (latency_ms, version, value)."""
    start = time.time()
    try:
        resp = requests.get(f"{url}/get", params={"key": key}, timeout=10)
        latency = (time.time() - start) * 1000
        if resp.status_code == 200:
            data = resp.json()
            return latency, data.get("version", 0), data.get("value", "")
        return latency, -1, ""
    except Exception:
        return (time.time() - start) * 1000, -1, ""


def run_single_test(config_name, config, write_ratio):
    """Run a single load test with the given config and write ratio."""
    tracker = VersionTracker()
    results = {
        "config": config_name,
        "write_ratio": write_ratio,
        "read_ratio": 1 - write_ratio,
        "write_latencies": [],
        "read_latencies": [],
        "stale_reads": 0,
        "total_reads": 0,
        "total_writes": 0,
        "rw_timestamps": [],  # (key, operation, timestamp) for time interval analysis
    }

    # Generate operation sequence
    operations = []
    for _ in range(NUM_REQUESTS):
        key = f"{KEY_PREFIX}{random.randint(0, NUM_KEYS - 1)}"
        if random.random() < write_ratio:
            operations.append(("write", key))
        else:
            operations.append(("read", key))

    # Determine URLs
    if config_name.startswith("leaderless"):
        all_nodes = config["read_urls"]
    else:
        all_nodes = None

    def execute_op(op_type, key):
        ts = time.time()

        if op_type == "write":
            value = f"v_{int(ts * 1000)}"
            if config_name.startswith("leaderless"):
                url = random.choice(all_nodes)
            else:
                url = config["write_url"]

            latency, version = do_write(url, key, value)
            if version > 0:
                tracker.update(key, version)
            return ("write", key, latency, version, ts)

        else:  # read
            if config_name.startswith("leaderless"):
                url = random.choice(all_nodes)
            else:
                url = random.choice(config["read_urls"])

            # For stale read detection on leader-follower,
            # also read from a random follower directly
            if config["follower_urls"] and random.random() < 0.5:
                url = random.choice(config["follower_urls"])

            latency, version, value = do_read(url, key)
            is_stale = tracker.is_stale(key, version) if version > 0 else False
            return ("read", key, latency, version, ts, is_stale)

    # Execute operations with thread pool
    with ThreadPoolExecutor(max_workers=NUM_THREADS) as pool:
        futures = []
        for op_type, key in operations:
            futures.append(pool.submit(execute_op, op_type, key))

        for future in as_completed(futures):
            try:
                result = future.result()
                if result[0] == "write":
                    _, key, latency, version, ts = result
                    results["write_latencies"].append(latency)
                    results["total_writes"] += 1
                    results["rw_timestamps"].append((key, "write", ts))
                else:
                    _, key, latency, version, ts, is_stale = result
                    results["read_latencies"].append(latency)
                    results["total_reads"] += 1
                    results["rw_timestamps"].append((key, "read", ts))
                    if is_stale:
                        results["stale_reads"] += 1
            except Exception as e:
                print(f"  Error: {e}")

    return results


def compute_stats(latencies):
    """Compute percentile statistics for a list of latencies."""
    if not latencies:
        return {}
    s = sorted(latencies)
    return {
        "count": len(s),
        "min": round(s[0], 2),
        "p50": round(s[len(s) // 2], 2),
        "p95": round(s[int(len(s) * 0.95)], 2),
        "p99": round(s[int(len(s) * 0.99)], 2),
        "max": round(s[-1], 2),
        "avg": round(sum(s) / len(s), 2),
    }


def compute_rw_intervals(rw_timestamps):
    """
    Compute time intervals between reads and writes of the same key.
    This shows how "local-in-time" our data generation is.
    """
    # Group by key
    by_key = {}
    for key, op, ts in rw_timestamps:
        if key not in by_key:
            by_key[key] = []
        by_key[key].append((op, ts))

    intervals = []
    for key, ops in by_key.items():
        ops.sort(key=lambda x: x[1])  # sort by timestamp
        for i in range(1, len(ops)):
            interval_ms = (ops[i][1] - ops[i - 1][1]) * 1000
            intervals.append(interval_ms)

    return intervals


# ============================================================
# Main
# ============================================================

def main():
    os.makedirs("results", exist_ok=True)
    all_results = []

    for config_name, config in CONFIGS.items():
        print(f"\n{'='*60}")
        print(f"Config: {config_name}")
        print(f"Compose: {config['compose_file']}")
        if config["env"]:
            env_str = " ".join(f"{k}={v}" for k, v in config["env"].items())
            print(f"Env: {env_str}")
        print(f"{'='*60}")

        input(f"\nPlease start: docker-compose -f {config['compose_file']} up --build")
        input("Press Enter when the cluster is ready...")

        for write_ratio in WRITE_RATIOS:
            read_ratio = 1 - write_ratio
            print(f"\n  Running: {int(write_ratio*100)}% writes / {int(read_ratio*100)}% reads ...")

            result = run_single_test(config_name, config, write_ratio)
            result["rw_intervals"] = compute_rw_intervals(result["rw_timestamps"])

            # Remove raw timestamps from saved data (too verbose)
            del result["rw_timestamps"]

            write_stats = compute_stats(result["write_latencies"])
            read_stats = compute_stats(result["read_latencies"])

            print(f"    Writes: {result['total_writes']}, Reads: {result['total_reads']}")
            print(f"    Write latency: {write_stats}")
            print(f"    Read  latency: {read_stats}")
            print(f"    Stale reads: {result['stale_reads']}/{result['total_reads']}")

            all_results.append(result)

        input("\nDone with this config. Stop the cluster (Ctrl+C), then press Enter...")

    # Save all results
    output_file = "results/load_test_results.json"
    with open(output_file, "w") as f:
        json.dump(all_results, f, indent=2)
    print(f"\nResults saved to {output_file}")
    print("Now run: python tests/generate_graphs.py")


if __name__ == "__main__":
    main()
