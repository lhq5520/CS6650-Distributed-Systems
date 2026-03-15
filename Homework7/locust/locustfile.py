from locust import HttpUser, task, between
import json
import random

SAMPLE_ITEMS = [
    {"product_id": "PROD-001", "quantity": 1, "price": 29.99},
    {"product_id": "PROD-002", "quantity": 2, "price": 49.99},
    {"product_id": "PROD-003", "quantity": 1, "price": 9.99},
]

def make_order_payload():
    return {
        "customer_id": random.randint(1000, 9999),
        "items": random.sample(SAMPLE_ITEMS, k=random.randint(1, 3))
    }

# ── Phase 1: Sync load test ────────────────────────────────────────────────────
class SyncOrderUser(HttpUser):
    """
    Phase 1 - Normal:  locust -f locustfile.py SyncOrderUser --users 5  --spawn-rate 1 --run-time 30s
    Phase 1 - Flash:   locust -f locustfile.py SyncOrderUser --users 20 --spawn-rate 10 --run-time 60s
    """
    wait_time = between(0.1, 0.5)

    @task
    def place_order_sync(self):
        self.client.post(
            "/orders/sync",
            json=make_order_payload(),
            name="/orders/sync"
        )

# ── Phase 3+: Async load test ──────────────────────────────────────────────────
class AsyncOrderUser(HttpUser):
    """
    Phase 3 - Flash:   locust -f locustfile.py AsyncOrderUser --users 20 --spawn-rate 10 --run-time 60s
    Phase 5 - Flash:   same command, adjust NUM_WORKERS env on ECS side
    """
    wait_time = between(0.1, 0.5)

    @task
    def place_order_async(self):
        with self.client.post(
            "/orders/async",
            json=make_order_payload(),
            name="/orders/async",
            catch_response=True
        ) as resp:
            # 202 Accepted is the expected success response
            if resp.status_code == 202:
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")
