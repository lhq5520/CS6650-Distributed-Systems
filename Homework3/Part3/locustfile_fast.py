from locust import FastHttpUser, task, between


class AlbumUser(FastHttpUser):
    wait_time = between(1, 2)

    @task(3)  # GET  tasks ratios 3
    def get_albums(self):
        self.client.get("/albums")

    @task(1)  # POST  tasks ratios 1 (3:1)
    def post_album(self):
        self.client.post("/albums", json={
            "id": "4",
            "title": "Test Album",
            "artist": "Test Artist",
            "price": 29.99
        })
