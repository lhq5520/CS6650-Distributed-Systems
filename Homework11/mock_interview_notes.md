# HW11 Mock Interview Notes - Album Store API

## Basic Info
- **Name:** Weifan Li | **Score:** 179/190 | **~2 submissions** to pass all critical scenarios
- **Tech Stack:** Go + PostgreSQL (RDS) + Amazon S3 + Terraform
- **Deployment:** Single EC2 (c6i.xlarge), port 80, Elastic IP, all in us-west-2a

---

## Q1: How many submissions? Most common failure?

About 2 submissions. First submission already passed all critical scenarios S1-S5.

Most common failure was in **load testing** -- S14 (Mixed Metadata + Uploads) and S15 (Large Payload Upload). Root cause: concurrent S3 uploads caused memory pressure or disk exhaustion.

---

## Q2: Where are photos stored? Why?

**Amazon S3**, same region as ChaosArena (us-west-2).

Three reasons:
1. **Durable storage** with publicly accessible URLs (spec requirement)
2. **Same region** minimizes network latency
3. **VPC Gateway Endpoint** keeps S3 traffic on AWS internal network, never hits public internet

---

## Q3: Deployment setup?

Very simple, single-node architecture:

```
Client --> EC2 (c6i.xlarge, Go binary on port 80, Elastic IP)
              |---> RDS PostgreSQL (metadata)
              |---> S3 (photo files)
```

All three in **us-west-2a** (same AZ) to minimize inter-service latency. Infrastructure managed with **Terraform**.

---

## Q4: Reverse proxy or load balancer?

**No.** Intentional decision.

Go HTTP server listens directly on port 80. I skipped the load balancer/reverse proxy to **eliminate an extra network hop** -- every millisecond of latency affects the load test score. Single instance was sufficient for the workload.

---

## Q5: How does background worker get notified?

**No external queue, no polling.** I use a Go goroutine spawned directly from the POST handler.

Flow:
1. POST handler reads the file
2. Assigns a seq number (in the handler, not the worker)
3. Returns **202 Accepted** immediately
4. Spawns a **goroutine** that uploads to S3 and updates the database

Key detail: a **semaphore** (buffered channel, size 20) limits concurrent uploads to control memory pressure. This avoids the 20-50ms overhead of SQS.

---

## Q6: Why assign seq in POST handler? How ensure correctness?

**Why in the handler:**
- Client needs a correct, unique seq in the 202 response **immediately**
- If the worker assigned it, two concurrent uploads could race and get the same seq

**How I ensure correctness:**
- Single PostgreSQL transaction:
  - `UPDATE albums SET next_seq = next_seq + 1 WHERE album_id = $1 RETURNING next_seq`
  - Then `INSERT` the photo row
- PostgreSQL's **row-level locking** on the UPDATE guarantees atomicity even under high concurrency

---

## Q7: What if the worker crashes mid-processing?

Two cases:

| Scenario | What happens |
|----------|-------------|
| S3 upload fails | Goroutine catches error, sets status to **"failed"** in DB + in-memory cache |
| Entire process crashes (OOM kill) | Photo stays in **"processing"** forever. Systemd restarts service within 3s, but orphaned records are NOT cleaned up |

**Production improvement:** would add a periodic **reconciliation job** to clean up stuck "processing" records.

---

## Q8: Database schema?

Two tables:

**albums:**
| Column | Type | Notes |
|--------|------|-------|
| album_id | UUID | PK |
| title | text | |
| description | text | |
| owner | text | |
| **next_seq** | integer | Per-album photo counter |

**photos:**
| Column | Type | Notes |
|--------|------|-------|
| photo_id | UUID | PK |
| album_id | UUID | FK to albums |
| seq | integer | |
| status | text | processing / completed / failed |
| url | text | S3 URL |
| created_at | timestamp | |

Key design: `next_seq` on albums table enables atomic seq allocation in a single `UPDATE...RETURNING` -- no separate counter table needed.

---

## Q9: Database indexes?

Two indexes beyond primary keys:

1. **`idx_photos_album`** on `photos(album_id)` -- speeds up "get all photos in album" queries
2. **`idx_photos_album_photo`** on `photos(album_id, photo_id)` -- optimizes GET and DELETE endpoints, enables **index-only lookups**

---

## Q10: Hardest load testing scenario? What bottleneck?

**S14 (Mixed Metadata + Uploads)** was the hardest.

- Initially: unbounded goroutines for S3 uploads
- Under concurrent load: **extreme GC pressure** -- Go's garbage collector scanning hundreds of large byte slices
- Result: upload **p95 reached 62 seconds**
- Fix: added **semaphore (cap = 20)** --> S14 went from **0 points to 15 (full score)**

---

## Q11: Single most impactful change?

**The semaphore** (buffered channel, size 20) to limit concurrent S3 uploads.

Impact:
- S14: **0/15 --> 15/15**
- Overall score: **163 --> 179**
- Controlled memory + GC pressure without sacrificing throughput

---

## Q12: How handle concurrent writes?

| Operation | Mechanism |
|-----------|-----------|
| Album creates | `INSERT ... ON CONFLICT DO UPDATE` (UPSERT) -- handles concurrent PUTs atomically |
| Photo seq allocation | `UPDATE ... RETURNING` in a transaction with **row-level locking** -- each upload gets unique seq |
| HTTP concurrency | Go handles natively with goroutines -- no external synchronization needed |

---

## Q13: Describe a specific bug (S9 Delete Race)

**This is a great storytelling question -- tell it step by step:**

**Symptom:** S9 (Delete Before Complete) failed. Event log showed: after deleting a photo, a GET 2 seconds later returned **200 instead of 404**.

**Root cause:** The upload goroutine was **still running** after the DELETE. When it finished the S3 upload, it **wrote the photo back** to both the database and the in-memory cache. The deleted photo was "resurrected."

**Fix (two changes):**
1. Added `WHERE status = 'processing'` to the UPDATE query -- a deleted photo's row **can never be resurrected**
2. Only update the in-memory cache when the DB update **actually affected a row** (rows affected > 0)

---

## Q14: How did you test locally?

- **curl** each endpoint on EC2 localhost:
  - `curl http://localhost/health`
  - `curl -X PUT http://localhost/albums/<uuid>`
  - etc.
- **systemctl status** to check service is running
- **journalctl** logs to verify DB connection and catch errors

---

## Q15: If you had one more week?

Migrate from standard S3 to **S3 Express One Zone**:
- **Single-digit millisecond latency** (roughly 10x faster than standard S3)
- The remaining 11 points (S12 and S15) are **entirely bottlenecked by S3 upload latency**
- This single change could push the score **close to 190**

---

## Q16: How did YOU add value over Claude?

Claude **cannot** interact with AWS directly. I handled all real-world infrastructure:

- Configuring **AWS CLI credentials**
- Running **terraform apply**
- Creating/managing **SSH keys**, fixing Windows file permission issues
- **SCP** uploading binaries to EC2
- Diagnosing live issues: "address already in use" errors, disk space exhaustion
- Reading **journalctl** logs in real-time
- Making **iterative submit-and-tune decisions** based on ChaosArena results -- choosing which optimization to pursue next

---

## Key Themes to Emphasize

| Theme | Example |
|-------|---------|
| **Latency optimization** | No LB, no SQS, same AZ, VPC endpoint |
| **Concurrency control** | Semaphore, PG row-level locking, UPSERT |
| **Debugging ability** | S9 delete race -- symptom, root cause, fix |
| **Tradeoff awareness** | In-process goroutine vs SQS, no reconciliation job (acknowledged limitation) |
| **Measurable impact** | Semaphore: 163-->179, S14: 0-->15, p95: 62s-->normal |
