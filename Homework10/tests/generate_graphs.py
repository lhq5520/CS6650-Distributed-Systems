"""
Generate graphs from load test results.

Produces:
  1. Latency distribution for reads (per config, per write ratio)
  2. Latency distribution for writes (per config, per write ratio)
  3. Time intervals between reads/writes of the same key
  4. Stale reads summary bar chart

Usage:
  python tests/generate_graphs.py
"""

import json
import os
import matplotlib.pyplot as plt
import numpy as np


def load_results():
    with open("results/load_test_results.json", "r") as f:
        return json.load(f)


def plot_latency_distributions(results):
    """
    For each write ratio, plot read and write latency distributions
    across all 4 configs side by side.
    """
    write_ratios = sorted(set(r["write_ratio"] for r in results))
    configs = sorted(set(r["config"] for r in results))

    # Shorter display names for configs
    display_names = {
        "leader_W5_R1": "LF W=5,R=1",
        "leader_W1_R5": "LF W=1,R=5",
        "leader_W3_R3": "LF W=3,R=3",
        "leaderless_WN_R1": "LL W=N,R=1",
    }

    for wr in write_ratios:
        fig, axes = plt.subplots(1, 2, figsize=(14, 5))
        fig.suptitle(f"Latency Distribution — {int(wr*100)}% Writes / {int((1-wr)*100)}% Reads",
                     fontsize=14, fontweight="bold")

        # Collect data for this write ratio
        for config in configs:
            r = next((x for x in results if x["config"] == config and x["write_ratio"] == wr), None)
            if not r:
                continue
            label = display_names.get(config, config)

            # Read latency histogram
            if r["read_latencies"]:
                axes[0].hist(r["read_latencies"], bins=30, alpha=0.6, label=label)

            # Write latency histogram
            if r["write_latencies"]:
                axes[1].hist(r["write_latencies"], bins=30, alpha=0.6, label=label)

        axes[0].set_title("Read Latency")
        axes[0].set_xlabel("Latency (ms)")
        axes[0].set_ylabel("Count")
        axes[0].legend()

        axes[1].set_title("Write Latency")
        axes[1].set_xlabel("Latency (ms)")
        axes[1].set_ylabel("Count")
        axes[1].legend()

        plt.tight_layout()
        filename = f"results/latency_{int(wr*100)}pct_writes.png"
        plt.savefig(filename, dpi=150)
        plt.close()
        print(f"  Saved {filename}")


def plot_rw_intervals(results):
    """
    Plot distribution of time intervals between reads/writes of the same key.
    This shows how "local-in-time" the test data is.
    """
    fig, axes = plt.subplots(2, 2, figsize=(14, 10))
    fig.suptitle("Time Intervals Between R/W Operations on Same Key", fontsize=14, fontweight="bold")

    write_ratios = sorted(set(r["write_ratio"] for r in results))

    for idx, wr in enumerate(write_ratios):
        ax = axes[idx // 2][idx % 2]

        # Combine intervals from all configs for this write ratio
        all_intervals = []
        for r in results:
            if r["write_ratio"] == wr and r.get("rw_intervals"):
                all_intervals.extend(r["rw_intervals"])

        if all_intervals:
            ax.hist(all_intervals, bins=40, alpha=0.7, color="steelblue")

        ax.set_title(f"{int(wr*100)}% Writes / {int((1-wr)*100)}% Reads")
        ax.set_xlabel("Interval (ms)")
        ax.set_ylabel("Count")

    plt.tight_layout()
    filename = "results/rw_intervals.png"
    plt.savefig(filename, dpi=150)
    plt.close()
    print(f"  Saved {filename}")


def plot_stale_reads_summary(results):
    """
    Bar chart showing stale read counts for each config x write ratio.
    """
    configs = sorted(set(r["config"] for r in results))
    write_ratios = sorted(set(r["write_ratio"] for r in results))

    display_names = {
        "leader_W5_R1": "LF W=5,R=1",
        "leader_W1_R5": "LF W=1,R=5",
        "leader_W3_R3": "LF W=3,R=3",
        "leaderless_WN_R1": "LL W=N,R=1",
    }

    x = np.arange(len(configs))
    width = 0.2

    fig, ax = plt.subplots(figsize=(12, 6))

    for i, wr in enumerate(write_ratios):
        stale_counts = []
        for config in configs:
            r = next((x for x in results if x["config"] == config and x["write_ratio"] == wr), None)
            stale_counts.append(r["stale_reads"] if r else 0)
        ax.bar(x + i * width, stale_counts, width,
               label=f"{int(wr*100)}%W / {int((1-wr)*100)}%R")

    ax.set_title("Stale Reads by Configuration and Write Ratio", fontsize=14, fontweight="bold")
    ax.set_xlabel("Configuration")
    ax.set_ylabel("Stale Read Count")
    ax.set_xticks(x + width * 1.5)
    ax.set_xticklabels([display_names.get(c, c) for c in configs])
    ax.legend()

    plt.tight_layout()
    filename = "results/stale_reads.png"
    plt.savefig(filename, dpi=150)
    plt.close()
    print(f"  Saved {filename}")


def main():
    print("Loading results...")
    results = load_results()

    print("Generating latency distribution graphs...")
    plot_latency_distributions(results)

    print("Generating R/W interval graphs...")
    plot_rw_intervals(results)

    print("Generating stale reads summary...")
    plot_stale_reads_summary(results)

    print("\nAll graphs saved to results/ folder!")


if __name__ == "__main__":
    main()
