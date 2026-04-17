# HW11 Presentation Script — Album Store API

> Organized by logical flow: architecture → code → core flow → concurrency → bugs → optimization.

---

## Part 1: Project Overview (30 seconds)

This project is an Album Store API. Users can create albums, upload photos, poll photo status, and delete photos. Photo upload is **asynchronous** — the client POSTs a photo, gets back 202 Accepted immediately, and a background goroutine uploads to S3.

- **Tech stack:** Go + PostgreSQL + Amazon S3
- **Infrastructure as Code:** Terraform
- **Final score:** 179 / 190 on ChaosArena load test
- Passed all critical scenarios (S1–S5) on first submission

---

## Part 2: Architecture (1 min)

```
                         ┌──────────────────────────────────┐
    Client ──HTTP──►     │  EC2 (c6i.xlarge)                │
                         │  Go binary, port 80, Elastic IP  │
                         │                                  │
                         │  POST /albums/:id/photos          │
                         │    ├─ sync: allocate seq, write DB│
                         │    ├─ return 202                  │
                         │    └─ async goroutine ──►  S3     │
                         └──────┬───────────────────────┬───┘
                                │                       │
                         VPC internal            VPC Gateway
                                │               Endpoint
                                ▼                       ▼
                         ┌──────────┐            ┌──────────┐
                         │ RDS      │            │ S3       │
                         │ Postgres │            │ Bucket   │
                         └──────────┘            └──────────┘

            All in us-west-2a (same AZ) to minimize network latency
```

**Key design decisions:**

- **Single EC2, no Load Balancer / Reverse Proxy** — eliminates an extra network hop. Go handles concurrency natively with goroutines, one instance is enough.
- **S3 VPC Gateway Endpoint** — S3 traffic stays on AWS internal network, never hits public internet, lower latency.
- **Same AZ (us-west-2a)** — EC2, RDS, and S3 all in the same availability zone for minimal inter-service latency.

---

## Part 3: Terraform Infrastructure (1 min)

All infrastructure managed in the `terraform/` directory:

| File | Purpose |
|------|---------|
| `vpc.tf` | VPC, public/private subnets, Internet Gateway, **S3 VPC Endpoint** |
| `ec2.tf` | EC2 instance + IAM Role (S3 access) + Elastic IP + systemd service |
| `rds.tf` | RDS PostgreSQL instance |
| `s3.tf` | S3 bucket for photo storage |
| `security.tf` | Security Groups controlling EC2 ↔ RDS network access |

Key points:
- EC2 **user_data** script auto-creates `/opt/album-store/.env` (env vars) and a systemd service file
- systemd configured with `Restart=always, RestartSec=3` — auto-restarts within 3 seconds if process crashes
- EC2 uses an **IAM Instance Profile** to access S3 — no hardcoded AWS credentials

---

## Part 4: Code Structure (1 min)

```
album-store/
├── cmd/server/main.go          ← Entry point: start server, register routes, run DB migration
├── internal/
│   ├── config/config.go        ← Read config from env vars (PORT, DATABASE_URL, S3_BUCKET)
│   ├── model/model.go          ← Data structs: Album, Photo, PhotoAccepted, ErrorResponse
│   ├── handler/
│   │   ├── album.go            ← PUT/GET/LIST album HTTP handlers
│   │   └── photo.go            ← POST upload / GET status / DELETE photo handlers
│   ├── store/
│   │   ├── postgres.go         ← DB connection pool (pgx, max=20, min=10)
│   │   ├── album_repo.go       ← Album CRUD (with UPSERT)
│   │   └── photo_repo.go       ← Photo CRUD + seq allocation (core concurrency logic)
│   ├── storage/
│   │   └── s3.go               ← S3 upload/delete (aws-sdk-go-v2 multipart uploader)
│   └── worker/
│       └── pool.go             ← PhotoCache (in-memory cache for fast GET status polling)
```

Using **chi** (lightweight Go HTTP router) and **pgx** (fastest PostgreSQL driver for Go).

---

## Part 5: Core Flow — Photo Upload (2 min, KEY SECTION)

This is the most important part of the system. Understanding this flow covers most of the design.

### 5.1 Request arrives

`photo.go:Upload()` — receives `POST /albums/:id/photos`

1. Parse multipart form, read photo data into `[]byte`
2. Generate a UUID as `photo_id`

### 5.2 Allocate seq number (synchronous, in the handler)

Calls `photo_repo.go:AllocateSeqAndInsert()` — **this is the key to concurrency correctness**

```go
// Single PostgreSQL transaction does two things:
tx.QueryRow(`UPDATE albums SET next_seq = next_seq + 1
             WHERE album_id = $1 RETURNING next_seq`, albumID)
tx.Exec(`INSERT INTO photos (photo_id, album_id, seq, status)
         VALUES ($1, $2, $3, 'processing')`, photoID, albumID, seq)
tx.Commit()
```

- `UPDATE ... RETURNING` leverages PostgreSQL's **row-level locking** — even with 100 concurrent uploads to the same album, each gets a unique, incrementing seq
- Must happen in the handler (not the background worker) because the 202 response needs to return the correct seq immediately

### 5.3 Write cache + return 202

```go
h.cache.Set(&CachedPhoto{PhotoID: photoID, Seq: seq, Status: "processing"})
writeJSON(w, 202, PhotoAccepted{PhotoID: photoID, Seq: seq, Status: "processing"})
```

### 5.4 Async S3 upload (goroutine)

```go
go h.processUpload(photoID, albumID, seq, data)
```

Inside `processUpload()`:

```go
uploadSem <- struct{}{}         // acquire semaphore (max 20 concurrent)
defer func() { <-uploadSem }() // release semaphore

url, err := h.s3.Upload(ctx, key, bytes.NewReader(data), "image/jpeg")
// success → update DB: status = "completed", url = S3 URL
// failure → update DB: status = "failed"
```

**The semaphore is the single most important optimization** — `var uploadSem = make(chan struct{}, 20)` limits concurrent S3 uploads to 20, preventing memory explosion and GC pressure.

### 5.5 Client polls status

`GET /albums/:id/photos/:photo_id` — checks in-memory cache first (O(1)), falls back to DB on cache miss.

---

## Part 6: Concurrency Control (1 min)

| Scenario | Mechanism | Code location |
|----------|-----------|---------------|
| Concurrent photo uploads to same album | PG transaction + `UPDATE ... RETURNING` + row-level lock | `photo_repo.go:22-48` |
| Concurrent PUT to same album | `INSERT ... ON CONFLICT DO UPDATE` (UPSERT) | `album_repo.go:23-29` |
| Limit concurrent S3 uploads | Buffered channel as semaphore, cap=20 | `photo.go:20` |
| Concurrent cache reads/writes | `sync.RWMutex` | `pool.go:22-55` |

---

## Part 7: Key Bug — Delete Race Condition (1 min, good story)

**Scenario:** S9 test "Delete Before Complete" — upload a photo, then DELETE it before S3 upload finishes.

**Symptom:** After DELETE, a GET on that photo returns 200 instead of 404. The photo was "resurrected."

**Root cause:**
1. User calls DELETE → DB row deleted → cache entry deleted → returns 204
2. But the S3 upload goroutine is still running
3. Goroutine finishes uploading → runs `UPDATE photos SET status='completed'` → row is gone, no effect on DB
4. **The problem:** goroutine also unconditionally updates the cache → writes the deleted photo back into cache → GET reads cache and returns 200

**Fix (two changes):**

```go
// photo_repo.go — only update if status is still 'processing'
UPDATE photos SET status=$1, url=$2
WHERE photo_id=$3 AND status='processing'

// photo.go — only write cache if DB actually updated a row
updated, _ := h.photoRepo.UpdateStatus(ctx, photoID, "completed", url)
if updated {   // ← key: after DELETE this is false, cache won't be written
    h.cache.Set(...)
}
```

---

## Part 8: Performance Optimization Journey (1 min)

| Phase | Problem | Change | Score |
|-------|---------|--------|-------|
| V1 | Functionally correct, passed S1-S5 | Basic implementation | ~163 |
| V2 | S14: unbounded goroutines → GC explosion, p95=62s | `uploadSem = make(chan struct{}, 20)` | 163 → **179** |
| Future | S12, S15 still bottlenecked by S3 latency | Plan: S3 Express One Zone (10x faster) | Target → 190 |

**Biggest single-change impact:** adding the semaphore. S14 went from 0/15 → 15/15.

---

## Part 9: Database Design (30 seconds)

```sql
albums:  album_id (UUID PK), title, description, owner, next_seq (INT)
photos:  photo_id (UUID PK), album_id (FK), seq, status, url, created_at

-- Indexes
idx_photos_album        ON photos(album_id)            -- lookup all photos in an album
idx_photos_album_photo  ON photos(album_id, photo_id)  -- GET/DELETE by both IDs, index-only scan
```

`next_seq` lives on the albums table — atomic increment via `UPDATE ... RETURNING` in a single SQL statement. No separate counter table needed.

---

## Part 10: My Contribution vs Claude's (30 seconds)

**Claude wrote:** Go code, Terraform configs, SQL schema

**I did (things Claude can't do):**
- All AWS infrastructure operations: terraform apply, SSH, SCP binary uploads
- Debugging SSH key permission issues on Windows
- Reading journalctl logs in real-time to diagnose issues (disk exhaustion, "address in use")
- Making iterative optimization decisions based on each ChaosArena submission
- Architecture decisions: no LB, no SQS, semaphore size = 20

---

## Quick Reference: Common Follow-up Questions

| Question | One-liner |
|----------|-----------|
| Why Go instead of Java/Python? | Compiles to single binary, low memory footprint, native goroutine concurrency, minimal deployment |
| Why not use SQS? | Saves 20-50ms per message latency; in-process goroutine is sufficient for single-node |
| Why no Load Balancer? | Single instance is enough; saves one network hop; every ms matters for load test score |
| Why semaphore size = 20? | Empirical: large enough to saturate throughput, small enough to avoid OOM. c6i.xlarge has 4 vCPU / 8GB RAM |
| Why DB pool max = 20? | Matches semaphore count — avoids connection starvation or waste |
| How would you change this for production? | Add ALB + multiple EC2s, replace goroutine with SQS, add reconciliation job for orphaned "processing" records |
| What is a VPC Endpoint? | Routes S3 traffic through AWS internal network instead of public internet — lower latency, no extra cost |
