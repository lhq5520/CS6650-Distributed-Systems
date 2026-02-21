import random
from locust import FastHttpUser, task, between


COMMON_TERMS = [
    "alpha",
    "beta",
    "gamma",
    "delta",
    "epsilon",
    "electronics",
    "books",
    "home",
    "sports",
    "clothing",
]


class ProductSearchUser(FastHttpUser):
    # Minimal wait to keep pressure high while still realistic
    wait_time = between(0.01, 0.05)

    @task(10)
    def search_products(self):
        term = random.choice(COMMON_TERMS)
        with self.client.get(
            f"/products/search?q={term}",
            name="GET /products/search",
            timeout=10,
            catch_response=True,
        ) as response:
            if response.status_code != 200:
                response.failure(f"Unexpected status: {response.status_code}")
                return
            try:
                body = response.json()
            except Exception as exc:
                response.failure(f"Invalid JSON: {exc}")
                return

            if not isinstance(body, dict) or "products" not in body or "total_found" not in body:
                response.failure("Missing expected response fields")

    @task(1)
    def health_check(self):
        self.client.get("/health", name="GET /health", timeout=5)
