"""
Locust load test for Crash & Recovery Demo
Usage:
  # Web UI mode (recommended for demo):
  locust -f locustfile.py --host http://localhost:8080

  # Headless mode:
  locust -f locustfile.py --host http://localhost:8080 --users 20 --spawn-rate 5 --run-time 3m --headless --csv=results
"""

from locust import HttpUser, task, between
import random
import json

SEARCH_TERMS = ["alpha", "beta", "gamma", "delta", "epsilon",
                "electronics", "books", "home", "sports", "clothing"]


class SearchUser(HttpUser):
    wait_time = between(0.1, 0.3)

    @task(10)
    def search_products(self):
        query = random.choice(SEARCH_TERMS)
        with self.client.get(
            f"/products/search?q={query}",
            name="/products/search",
            catch_response=True
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Status {resp.status_code}")

    @task(1)
    def check_health(self):
        self.client.get("/health", name="/health")

    @task(1)
    def check_metrics(self):
        with self.client.get("/metrics", name="/metrics", catch_response=True) as resp:
            if resp.status_code == 200:
                resp.success()