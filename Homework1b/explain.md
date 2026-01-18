# Performance Testing Reflection

After running the 30-second load test against my Go-Gin server on AWS EC2, I observed several interesting patterns that illustrate key concepts in distributed systems performance.

## Distribution and Tail Latency

My histogram clearly shows the classic "long tail" distribution the assignment predicted. The majority of requests clustered tightly between 25-32ms, with a strong peak around 27ms. However, approximately 10-15% of requests fell into the "slow" category, extending out to 50-55ms. This long tail is significant because even though most requests are fast, these outliers can severely impact user experience - a user hitting that 50ms response is experiencing nearly double the typical latency.

## Consistency and Patterns

The scatter plot reveals that response times remained remarkably consistent throughout the entire 30-second window, with no degradation over time. The outliers appear randomly distributed rather than clustered, suggesting they're not due to resource exhaustion or cold starts. This random nature of the slow requests is characteristic of systems running on shared infrastructure where you can't fully control when other processes compete for resources.

## Percentile Analysis

Based on my results, the median response time is approximately 28ms, while the 95th percentile sits around 38-40ms - about 35-40% higher. This gap demonstrates why percentile metrics are crucial in production systems. If I only looked at the average, I'd think my service was performing great at ~30ms, but 5% of my users would be experiencing noticeably slower responses. In production, SLAs are typically written around 95th or 99th percentile performance, not averages, because outliers matter.

## Infrastructure Limitations

Running this on a t2.micro instance definitely contributed to the variability I observed. These instances use burstable CPU credits and share physical hardware with other tenants. The occasional 50ms+ spikes likely occur when my instance is competing for CPU with neighboring workloads or when I've exhausted my CPU credits and get throttled. Additionally, the t2.micro has minimal RAM and a single vCPU, making it susceptible to any OS-level scheduling delays.

## Scaling Considerations

My current test sends requests sequentially - one completes before the next begins. This allowed me to process roughly 33 requests per second. If I suddenly had 100 concurrent users, the results would be dramatically different. The t2.micro would be completely overwhelmed, response times would likely jump to hundreds of milliseconds or even seconds, and I'd probably see timeout failures. This exercise shows why load testing with realistic concurrent users is essential before launching a service.

## Network vs. Processing Time

The 25-30ms baseline includes both network round-trip time and server processing. To isolate these factors, I could SSH into the EC2 instance and test locally with curl localhost:8080/albums, which would eliminate network latency and show pure processing time. Given that my service just returns a simple JSON array, the actual processing time is probably under 5ms, meaning most of my response time is network overhead. This is an important insight - for simple CRUD operations, network latency often dominates total response time.

## Key Takeaway

This exercise demonstrated that tail latency is a real phenomenon in distributed systems, not just a theoretical concern. Even on a simple GET endpoint with no database queries or complex logic, I still saw significant variability in response times. In production, understanding these performance characteristics is critical for capacity planning, setting realistic SLAs, and ensuring good user experience. The data shows that you must design for your worst-case scenarios (the tail), not your average case.
