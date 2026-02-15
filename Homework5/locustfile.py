from locust import HttpUser, task, between
from locust.contrib.fasthttp import FastHttpUser
import random
import json

PRODUCT_COUNT = 50


def make_product(pid):
    return {
        "product_id": pid,
        "sku": f"SKU-{pid:05d}",
        "manufacturer": f"Manufacturer-{pid}",
        "category_id": random.randint(1, 100),
        "weight": random.randint(100, 5000),
        "some_other_id": random.randint(1, 1000)
    }


# ==================== HttpUser ====================
class ProductHttpUser(HttpUser):
    wait_time = between(1, 3)

    def on_start(self):
        """Pre-populate products so GET requests don't 404"""
        for pid in range(1, PRODUCT_COUNT + 1):
            self.client.post(
                f"/products/{pid}/details",
                json=make_product(pid),
                name="/products/[id]/details (setup)"
            )

    @task(3)
    def get_product(self):
        """GET /products/{id} — reads are more common in real world"""
        pid = random.randint(1, PRODUCT_COUNT)
        self.client.get(
            f"/products/{pid}",
            name="/products/[id]"
        )

    @task(1)
    def create_product(self):
        """POST /products/{id}/details — writes are less common"""
        pid = random.randint(1, PRODUCT_COUNT)
        self.client.post(
            f"/products/{pid}/details",
            json=make_product(pid),
            name="/products/[id]/details"
        )


# ==================== FastHttpUser ====================
class ProductFastHttpUser(FastHttpUser):
    wait_time = between(1, 3)

    def on_start(self):
        for pid in range(1, PRODUCT_COUNT + 1):
            self.client.post(
                f"/products/{pid}/details",
                headers={"Content-Type": "application/json"},
                data=json.dumps(make_product(pid)),
                name="/products/[id]/details (setup)"
            )

    @task(3)
    def get_product(self):
        pid = random.randint(1, PRODUCT_COUNT)
        self.client.get(
            f"/products/{pid}",
            name="/products/[id]"
        )

    @task(1)
    def create_product(self):
        pid = random.randint(1, PRODUCT_COUNT)
        self.client.post(
            f"/products/{pid}/details",
            headers={"Content-Type": "application/json"},
            data=json.dumps(make_product(pid)),
            name="/products/[id]/details"
        )