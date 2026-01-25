What Happened?
Observed Results:

Initially: Both instances have the same 3 albums
POST to Instance 2: Successfully added Betty Carter (ID: 4)
Query again:

Instance 1: Still only has 3 albums (no Betty Carter)
Instance 2: Now has 4 albums (includes Betty Carter)

Why Did This Happen?
These two EC2 instances are completely independent:

Separate in-memory storage: Each instance stores data in its own memory slice
No shared database: They have zero communication or data synchronization
Independent state management: Changes to Instance 2 are completely unknown to Instance 1

In production, this causes serious issues:

User adds items to cart on Instance 2, load balancer routes them to Instance 1 → cart "disappears"
Inventory decremented on one server, but other servers don't know → overselling
User updates profile on one instance, but sees old data when hitting another instance

Solutions

Shared Database: MySQL, PostgreSQL (all instances read/write to same DB)
Distributed Cache: Redis, Memcached
Message Queues: Kafka, RabbitMQ (sync data between instances)
Distributed Consensus: Raft, Paxos algorithms

This is exactly what happened in the MapReduce paper - how to coordinate work across multiple machines while maintaining data consistency!

Key Talking away Points

CAP Theorem: Consistency, Availability, Partition Tolerance
Eventual Consistency vs Strong Consistency
Stateless vs Stateful Services
Why microservices need databases/caches instead of in-memory storage

This experiment perfectly illustrates why we need databases and distributed system theory!

---

```bash
Starting data test...
Instance 1 response: [
{
"id": "1",
"title": "Blue Train",
"artist": "John Coltrane",
"price": 56.99
},
{
"id": "2",
"title": "Jeru",
"artist": "Gerry Mulligan",
"price": 17.99
},
{
"id": "3",
"title": "Sarah Vaughan and Clifford Brown",
"artist": "Sarah Vaughan",
"price": 39.99
}
]

and...
Instance 2 response: [
{
"id": "1",
"title": "Blue Train",
"artist": "John Coltrane",
"price": 56.99
},
{
"id": "2",
"title": "Jeru",
"artist": "Gerry Mulligan",
"price": 17.99
},
{
"id": "3",
"title": "Sarah Vaughan and Clifford Brown",
"artist": "Sarah Vaughan",
"price": 39.99
}
]

and adding...
Instance 2 response: {
"id": "4",
"title": "The Modern Sound of Betty Carter",
"artist": "Betty Carter",
"price": 49.99
}
Instance 1 response: [
{
"id": "1",
"title": "Blue Train",
"artist": "John Coltrane",
"price": 56.99
},
{
"id": "2",
"title": "Jeru",
"artist": "Gerry Mulligan",
"price": 17.99
},
{
"id": "3",
"title": "Sarah Vaughan and Clifford Brown",
"artist": "Sarah Vaughan",
"price": 39.99
}
]

and...
Instance 2 response: [
{
"id": "1",
"title": "Blue Train",
"artist": "John Coltrane",
"price": 56.99
},
{
"id": "2",
"title": "Jeru",
"artist": "Gerry Mulligan",
"price": 17.99
},
{
"id": "3",
"title": "Sarah Vaughan and Clifford Brown",
"artist": "Sarah Vaughan",
"price": 39.99
},
{
"id": "4",
"title": "The Modern Sound of Betty Carter",
"artist": "Betty Carter",
"price": 49.99
}
]
uhoh... what happened?
```
