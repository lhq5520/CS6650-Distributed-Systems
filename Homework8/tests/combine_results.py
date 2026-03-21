"""
HW8 STEP III: Combine and analyze MySQL vs DynamoDB test results

Usage:
  python combine_results.py <mysql_results.json> <dynamodb_results.json>

Outputs:
  - combined_results.json
  - Comparison tables printed to console (copy into your report)
"""

import sys
import json
import statistics

def load_results(filepath):
    with open(filepath) as f:
        return json.load(f)

def calc_stats(results):
    times = [r["response_time"] for r in results if r["success"]]
    if not times:
        return {"avg": 0, "p50": 0, "p95": 0, "p99": 0, "success_rate": 0, "total": len(results)}
    times.sort()
    return {
        "avg":          round(statistics.mean(times), 2),
        "p50":          round(times[len(times) // 2], 2),
        "p95":          round(times[int(len(times) * 0.95)], 2),
        "p99":          round(times[int(len(times) * 0.99)], 2),
        "success_rate": round(100 * sum(1 for r in results if r["success"]) / len(results), 2),
        "total":        len(results)
    }

def winner(mysql_val, dynamo_val, lower_is_better=True):
    if lower_is_better:
        if mysql_val < dynamo_val:
            return "MySQL", round(dynamo_val - mysql_val, 2)
        elif dynamo_val < mysql_val:
            return "DynamoDB", round(mysql_val - dynamo_val, 2)
    else:
        if mysql_val > dynamo_val:
            return "MySQL", round(mysql_val - dynamo_val, 2)
        elif dynamo_val > mysql_val:
            return "DynamoDB", round(dynamo_val - mysql_val, 2)
    return "Tie", 0

def main():
    if len(sys.argv) != 3:
        print("Usage: python combine_results.py mysql_results.json dynamodb_results.json")
        sys.exit(1)

    mysql_data = load_results(sys.argv[1])
    dynamo_data = load_results(sys.argv[2])

    # Validate counts
    for name, data in [("MySQL", mysql_data), ("DynamoDB", dynamo_data)]:
        total = len(data)
        by_op = {}
        for r in data:
            by_op.setdefault(r["operation"], []).append(r)
        print(f"{name}: {total} total operations")
        for op, items in sorted(by_op.items()):
            print(f"  {op}: {len(items)}")
        if total != 150:
            print(f"  WARNING: Expected 150, got {total}")

    # Create combined file
    combined = {
        "mysql":    mysql_data,
        "dynamodb": dynamo_data,
        "metadata": {
            "mysql_count":    len(mysql_data),
            "dynamodb_count": len(dynamo_data),
            "generated_at":   __import__("datetime").datetime.now().isoformat()
        }
    }
    with open("combined_results.json", "w") as f:
        json.dump(combined, f, indent=2)
    print("\n-> Saved combined_results.json")

    # --- Overall Comparison Table ---
    m_stats = calc_stats(mysql_data)
    d_stats = calc_stats(dynamo_data)

    print("\n" + "=" * 75)
    print("PART 1: OVERALL PERFORMANCE COMPARISON")
    print("=" * 75)
    header = f"{'Metric':<28} {'MySQL':>10} {'DynamoDB':>10} {'Winner':>10} {'Margin':>10}"
    print(header)
    print("-" * 75)

    for label, m_key, lower in [
        ("Avg Response Time (ms)",  "avg", True),
        ("P50 Response Time (ms)",  "p50", True),
        ("P95 Response Time (ms)",  "p95", True),
        ("P99 Response Time (ms)",  "p99", True),
        ("Success Rate (%)",        "success_rate", False),
    ]:
        m_val = m_stats[m_key]
        d_val = d_stats[m_key]
        w, margin = winner(m_val, d_val, lower)
        print(f"{label:<28} {m_val:>10} {d_val:>10} {w:>10} {margin:>10}")

    print(f"{'Total Operations':<28} {'150':>10} {'150':>10} {'':>10} {'':>10}")

    # --- Operation-Specific Breakdown ---
    print("\n" + "=" * 75)
    print("PART 1: OPERATION-SPECIFIC BREAKDOWN")
    print("=" * 75)
    header2 = f"{'Operation':<16} {'MySQL Avg(ms)':>14} {'DynamoDB Avg(ms)':>17} {'Faster By':>12}"
    print(header2)
    print("-" * 65)

    for op in ["create_cart", "add_items", "get_cart"]:
        m_op = [r for r in mysql_data if r["operation"] == op]
        d_op = [r for r in dynamo_data if r["operation"] == op]
        m_s = calc_stats(m_op)
        d_s = calc_stats(d_op)
        w, margin = winner(m_s["avg"], d_s["avg"], True)
        print(f"{op.upper():<16} {m_s['avg']:>14} {d_s['avg']:>17} {w+' '+str(margin)+'ms':>12}")

    print("\n" + "=" * 75)
    print("DATA SOURCE: combined_results.json")
    print("Copy the tables above into your report!")
    print("=" * 75)

if __name__ == "__main__":
    main()
