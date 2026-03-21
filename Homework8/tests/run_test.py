"""
HW8 Performance Test Script
Runs exactly 150 operations: 50 create, 50 add_items, 50 get_cart
Saves results to mysql_test_results.json or dynamodb_test_results.json

Usage:
  python run_test.py <base_url> <output_file>

Example:
  python run_test.py http://1.2.3.4:5173 mysql_test_results.json
  python run_test.py http://1.2.3.4:5173 dynamodb_test_results.json
"""

import sys
import json
import time
import requests
from datetime import datetime, timezone

def run_test(base_url, output_file):
    base_url = base_url.rstrip("/")
    results = []
    cart_ids = []

    print(f"Target: {base_url}")
    print(f"Output: {output_file}")
    print("=" * 50)

    # --- Phase 1: Create 50 carts ---
    print("\n[Phase 1] Creating 50 carts...")
    for i in range(50):
        payload = {
            "customer_id": f"customer-{i+1:03d}",
            "customer_name": f"Test Customer {i+1}"
        }
        start = time.time()
        try:
            resp = requests.post(f"{base_url}/shopping-carts", json=payload, timeout=10)
            elapsed = (time.time() - start) * 1000  # ms

            result = {
                "operation": "create_cart",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 201,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).isoformat()
            }
            results.append(result)

            if resp.status_code == 201:
                data = resp.json()
                cart_ids.append(data["cart_id"])

            if (i + 1) % 10 == 0:
                print(f"  Created {i+1}/50  ({elapsed:.1f}ms)")
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "create_cart",
                "response_time": round(elapsed, 2),
                "success": False,
                "status_code": 0,
                "timestamp": datetime.now(timezone.utc).isoformat()
            })
            print(f"  ERROR creating cart {i+1}: {e}")

    print(f"  -> {len(cart_ids)} carts created successfully")

    if len(cart_ids) < 50:
        print(f"  WARNING: Only {len(cart_ids)}/50 carts created. Reusing for remaining tests.")

    # --- Phase 2: Add items to 50 carts ---
    print("\n[Phase 2] Adding items to 50 carts...")
    for i in range(50):
        cart_id = cart_ids[i % len(cart_ids)]
        payload = {
            "product_id": f"prod-{i+1:03d}",
            "product_name": f"Test Product {i+1}",
            "quantity": (i % 5) + 1,
            "unit_price": round(9.99 + (i * 0.5), 2)
        }
        start = time.time()
        try:
            resp = requests.post(
                f"{base_url}/shopping-carts/{cart_id}/items",
                json=payload, timeout=10
            )
            elapsed = (time.time() - start) * 1000

            results.append({
                "operation": "add_items",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 201,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).isoformat()
            })

            if (i + 1) % 10 == 0:
                print(f"  Added items {i+1}/50  ({elapsed:.1f}ms)")
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "add_items",
                "response_time": round(elapsed, 2),
                "success": False,
                "status_code": 0,
                "timestamp": datetime.now(timezone.utc).isoformat()
            })
            print(f"  ERROR adding item {i+1}: {e}")

    # --- Phase 3: Get 50 carts ---
    print("\n[Phase 3] Retrieving 50 carts...")
    for i in range(50):
        cart_id = cart_ids[i % len(cart_ids)]
        start = time.time()
        try:
            resp = requests.get(
                f"{base_url}/shopping-carts/{cart_id}",
                timeout=10
            )
            elapsed = (time.time() - start) * 1000

            results.append({
                "operation": "get_cart",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 200,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).isoformat()
            })

            if (i + 1) % 10 == 0:
                print(f"  Retrieved {i+1}/50  ({elapsed:.1f}ms)")
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "get_cart",
                "response_time": round(elapsed, 2),
                "success": False,
                "status_code": 0,
                "timestamp": datetime.now(timezone.utc).isoformat()
            })
            print(f"  ERROR getting cart {i+1}: {e}")

    # --- Save results ---
    with open(output_file, "w") as f:
        json.dump(results, f, indent=2)

    # --- Summary ---
    print("\n" + "=" * 50)
    print("SUMMARY")
    print("=" * 50)
    total = len(results)
    success = sum(1 for r in results if r["success"])
    times = [r["response_time"] for r in results if r["success"]]

    print(f"Total operations: {total}")
    print(f"Successful:       {success}/{total} ({100*success/total:.1f}%)")

    if times:
        times.sort()
        avg = sum(times) / len(times)
        p50 = times[len(times) // 2]
        p95 = times[int(len(times) * 0.95)]
        p99 = times[int(len(times) * 0.99)]
        print(f"Avg response:     {avg:.2f} ms")
        print(f"P50:              {p50:.2f} ms")
        print(f"P95:              {p95:.2f} ms")
        print(f"P99:              {p99:.2f} ms")

    for op in ["create_cart", "add_items", "get_cart"]:
        op_times = [r["response_time"] for r in results if r["operation"] == op and r["success"]]
        if op_times:
            print(f"\n  {op}: avg={sum(op_times)/len(op_times):.2f}ms  "
                  f"min={min(op_times):.2f}ms  max={max(op_times):.2f}ms")

    print(f"\nResults saved to: {output_file}")

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: python run_test.py <base_url> <output_file>")
        print("  e.g. python run_test.py http://1.2.3.4:5173 mysql_test_results.json")
        sys.exit(1)

    run_test(sys.argv[1], sys.argv[2])
