Part V: Claude Code Mystery Bug Analysis
1. Time Spent
Total time: 27 minutes

Understanding the code and identifying the bug: ~10 minutes
Learning about goroutines and concurrency concepts: ~10 minutes
Understanding AWS Lambda deployment: ~7 minutes

2. Mystery Bug Description
Bug Type: Race Condition (Concurrency Bug)
Location: main.go, lines 127-131 in the postAlbumCount function
Problem:
The function spawns 10,000 goroutines to increment a counter, but lacks synchronization:
gogo func() {
    defer wg.Done()
    current := albumCounts[index].Count  // Multiple goroutines read same value
    albumCounts[index].Count = current + 1  // Then overwrite each other
}()
```

**Evidence from System Logs (CloudWatch Logs: `/aws/lambda/album-counter`):**
```
[2026-01-24T23:46:00] Album ID: 1, Final Count: 3847
Expected: 10,000
Actual: ~3,000-5,000 (varies each time)
Root Cause:
Multiple goroutines read the same counter value before any write back, causing lost updates. Without a mutex to protect the critical section, concurrent reads and writes result in a final count far below 10,000.
Solution Applied:
Added sync.Mutex to protect the counter increment, ensuring only one goroutine modifies the count at a time.