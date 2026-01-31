## Locust Local Load Test

**Test Setup:**

- GET:POST ratio = 3:1
- 1 worker
- 50 users
- 10 users/sec spawn rate
- Target: Go REST API (Albums service)

**Results:**
| Metric | GET /albums | POST /albums |
|--------|-------------|--------------|
| Total Requests | 6,724 | 2,223 |
| Failures | 0 | 0 |
| Median (P50) | 8 ms | 2 ms |
| 95th %ile (P95) | 18 ms | 3 ms |
| Average | 8.83 ms | 2.29 ms |
| RPS | 22.7 | 7.3 |
| Avg Response Size | 132 KB | 93 bytes |

**Screenshots:**
![alt text](<locust (1).png>)
![alt text](<locust (2).png>)

**Analysis:**

Surprisingly, POST requests were ~4x faster than GET requests despite typically being more expensive operations. This counter-intuitive result is explained by response size differences:

- GET returns the entire albums array (132KB average)
- POST returns only the newly created album (93 bytes)
- The albums array grows continuously during the test (from 3 to 2000+ elements)

**GET vs POST difference:**

- GET uses `IndentedJSON()` which adds formatting overhead
- GET response size increases over time as array grows, explaining the gradual P95 increase from 6ms to 22ms
- POST payload is consistently small (single album object)

**System Performance:**

- 0% failure rate - server handled load well
- Stable ~30 RPS throughput
- Response times remained acceptable (<25ms)
- 3:1 request ratio maintained correctly

**Lesson learned:**
Response payload size matters more than operation type. In this test, a "write" operation (POST) outperformed a "read" operation (GET) due to data size differences. Real-world applications should use pagination for GET requests and databases instead of in-memory arrays to avoid unbounded growth.

## Amdahl's Law Experiment

**Test Setup:**

- Increased workers from 1 to 4
- Same test parameters: 50 users, 10/sec spawn rate, 3:1 GET:POST ratio
- Measured throughput (RPS) to verify if performance scales linearly

**Results Comparison:**

| Workers   | Total RPS | GET RPS | POST RPS | Speedup         |
| --------- | --------- | ------- | -------- | --------------- |
| 1 worker  | 30.0      | 22.7    | 7.3      | 1.0x (baseline) |
| 4 workers | 28.8      | 22.7    | 6.1      | 0.96x           |

**Screenshots:**
![alt text](<locust with 4 workers (1).png>)
![alt text](<locust with 4 workers (2).png>)

**Analysis:**

Contrary to expectations, increasing workers from 1 to 4 provided **zero performance improvement** - throughput actually decreased slightly (0.96x speedup). This is a textbook example of Amdahl's Law: when the serial portion of work dominates, adding more parallel workers provides no benefit.

**Why no speedup?**

The bottleneck is the shared `albums` array on the server:

- All requests (from all 4 workers) hit the **same single server instance**
- The albums array is a **global shared resource** (similar to the plain map in Collections experiment)
- Operations on this array are effectively **serialized** even with concurrent requests
- Adding Locust workers only increases client-side parallelism, not server-side capacity

**Amdahl's Law formula:**

```
Speedup = 1 / (Serial% + Parallel%/N)
If ~95% serial: 1 / (0.95 + 0.05/4) ≈ 1.04x theoretical max
Actual result: 0.96x - confirms extreme serialization
```

**Lesson learned:**

This experiment demonstrates that scaling load generators (workers) doesn't help if the server itself is the bottleneck. The shared albums array creates lock contention similar to the Mutex experiment - multiple threads competing for the same resource. To truly scale, we'd need: (1) thread-safe data structures (like sync.Map), (2) database with concurrent access support, or (3) multiple server instances behind a load balancer.

**Connection to Collections experiment:**

- Plain map + 50 goroutines = crashed (complete failure)
- Albums array + 4 workers = no speedup (Amdahl's Law limit)
- Both demonstrate that shared mutable state is the enemy of parallelism

## Context Switching - FastHttpUser Experiment

**Test Setup:**

- Replaced `HttpUser` with `FastHttpUser` in locustfile
- FastHttpUser uses C-based geventhttpclient (lower CPU overhead)
- Same parameters: 4 workers, 50 users, 10/sec spawn rate

**Results Comparison:**

| Client Type     | Total RPS | GET RPS | POST RPS | GET Median | POST Median |
| --------------- | --------- | ------- | -------- | ---------- | ----------- |
| HttpUser        | 28.8      | 22.7    | 6.1      | 8 ms       | 2 ms        |
| FastHttpUser    | 30.2      | 22.9    | 7.3      | 9 ms       | 1 ms        |
| **Improvement** | **+5%**   | +1%     | +20%     | -1 ms      | +1 ms       |

**Screenshots:**
![alt text](<locus with fasthttp (1).png>)
![alt text](<locus with fasthttp (2).png>)

**Analysis:**

Despite documentation suggesting 5x-6x performance improvements with FastHttpUser, we observed only a 5% throughput increase. This minimal improvement confirms that the bottleneck is server-side (the shared albums array), not client-side.

**What did we observe?**

- FastHttpUser uses less CPU on load generator (C-based vs Python)
- Total RPS remained ~30 (same bottleneck as before)
- Response time patterns identical to HttpUser (gradual P95 increase)
- The server's capacity limit unchanged

**Why minimal improvement?**

FastHttpUser optimizes the **client-side** by reducing context switching overhead (similar to single-thread goroutines in the Go experiment). However, our server is already saturated at ~30 RPS due to the serialized albums array access. Optimizing the non-bottleneck component provides negligible benefit.

**Connection to Go Context Switching experiment:**

- **FastHttpUser** (C-based, efficient) ≈ Single-thread goroutines (fast user-space switching)
- **HttpUser** (Python-based) ≈ Multi-thread goroutines (slower kernel-space switching)
- Both show that reducing switching overhead helps, but only if that's the bottleneck

**Lesson learned:**
Performance optimization must target the actual bottleneck. In this system, the bottleneck is server-side shared state (albums array), not client-side request generation. FastHttpUser would shine in scenarios where the client CPU is maxed out, but here the server reaches capacity first. This reinforces Amdahl's Law: optimizing the parallel portion (client) doesn't help when the serial portion (server) dominates.
