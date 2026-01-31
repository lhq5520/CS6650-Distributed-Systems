# CS6650 Homework 3 Report

**Date:** January 30, 2026

---

## Part I: Paper review

It's on Piazza

---

## Part II: Thread Experiments

### Synchronization Comparison

| Experiment        | Result                            | Key Finding                                   |
| ----------------- | --------------------------------- | --------------------------------------------- |
| 1. Atomicity      | Atomic: 50000, Non-atomic: ~48000 | `ops++` loses updates; use `atomic.Add()`     |
| 2. Plain Map      | Crashed                           | Go maps not thread-safe                       |
| 3. Mutex          | 5.347ms, 50000 entries            | Serializes access, prevents crashes           |
| 4. RWMutex        | 5.448ms, 50000 entries            | No benefit for write-only workload            |
| 5. Sync.Map       | 2.302ms, 50000 entries            | **2.3x faster** - optimized for disjoint keys |
| 6. File I/O       | Unbuffered: 315ms, Buffered: 10ms | **30x speedup** with buffering                |
| 7. Context Switch | Single: 89ns, Multi: 151ns        | User-space switching 1.7x faster              |

**Screenshots:** In Part 2 folder

---

## Part III: Load Testing with Locust

### Test Results Summary

| Experiment   | Setup              | RPS                  | Key Finding                                        |
| ------------ | ------------------ | -------------------- | -------------------------------------------------- |
| Local Test   | 1 worker, 50 users | GET: 22.7, POST: 7.3 | POST 4x faster (93 bytes vs 132KB response)        |
| Amdahl's Law | 4 workers          | 28.8 (0.96x speedup) | No improvement - server bottleneck                 |
| FastHttpUser | 4 workers          | 30.2 (+5%)           | Client optimization doesn't help server bottleneck |

**Screenshots:** In Part 3 folder

---

## Key Takeaways

**Central theme:** Shared mutable state is the enemy of scalability

**Part II lessons:**

- Proper synchronization required for concurrent access
- Choose mechanism based on workload (Mutex/RWMutex/Sync.Map)
- Buffered I/O critical for performance (30x gain)
- Goroutines have minimal switching cost

**Part III lessons:**

- Response size matters more than operation type
- Amdahl's Law: Serial bottlenecks limit parallel speedup
- Optimize actual bottlenecks, not peripheral components
- Shared state (albums array) prevented scaling

**Connections:**

- Plain map crash ↔ Albums array serialization
- Mutex serialization ↔ Amdahl's Law limits
- Context switching costs ↔ FastHttpUser efficiency

---
