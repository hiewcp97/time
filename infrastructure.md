# Infrastructure Design

## Overview

The Retention Agent Platform is designed to support:

- 500,000+ customers
- Customer search, filtering, sorting, and pagination
- Individual pitch generation
- Bulk pitch generation
- LLM integration
- Horizontal scaling
- Production observability

---

# High-Level Architecture

```text
                        +-------------------+
                        | Retention Agents  |
                        +---------+---------+
                                  |
                                  v
                        +-------------------+
                        |  Load Balancer    |
                        +---------+---------+
                                  |
                    +-------------+-------------+
                    |                           |
                    v                           v
          +------------------+      +------------------+
          |   API Service    |      |   API Service    |
          |   Instance #1    |      |   Instance #2    |
          +--------+---------+      +--------+---------+
                   |                         |
                   +------------+------------+
                                |
                                v
                     +----------------------+
                     |       Redis          |
                     | Cache / Rate Limits  |
                     +-----------+----------+
                                 |
                +----------------+----------------+
                |                                 |
                v                                 v
      +-------------------+          +-------------------+
      |   PostgreSQL      |          | Background Worker |
      |    Primary DB     |          |     Pool          |
      +---------+---------+          +---------+---------+
                |                              |
                |                              |
                v                              v
      +-------------------+          +-------------------+
      | PostgreSQL Read   |          |    LLM Provider   |
      |     Replica       |          | OpenAI / Gemini   |
      +-------------------+          +-------------------+
```

---

# Components

## API Service

Responsibilities:

- Customer search
- Customer details retrieval
- Create pitch generation requests
- Create bulk generation jobs
- Job status retrieval

Technology:

- Go
- Gin Framework
- pgx PostgreSQL driver

Scaling:

- Stateless
- Horizontally scalable

---

## PostgreSQL

Stores:

- Customers
- Usage history
- Generated pitches
- Bulk jobs
- Job items

Why PostgreSQL?

- Strong consistency
- Mature indexing capabilities
- Excellent query planner
- Supports partitioning
- Supports full-text search

---

## Redis

Used for:

### Pitch Cache

```text
pitch:{customer_id}:{hash}
```

Purpose:

Avoid regenerating pitches when customer data has not changed.

### Rate Limiting

Protect LLM API from bursts.

### Distributed Locks (Future)

Prevent duplicate bulk processing.

---

## Worker Pool

Responsible for:

- Bulk pitch generation
- Retry handling
- LLM requests

Workers process jobs asynchronously.

Benefits:

- Prevents API timeouts
- Protects LLM provider
- Provides backpressure control

---

## LLM Provider

Supported providers:

- OpenAI
- Gemini
- Claude

Responsibilities:

- Generate personalized recontract pitches

Prompt contains:

- Customer plan
- Tenure
- Usage history
- Contract information

---

# Database Scaling Strategy

## Current Scale (500k Customers)

Single PostgreSQL instance.

Indexes:

```sql
CREATE INDEX idx_customer_contract_end
ON customers(contract_end_date);

CREATE INDEX idx_customer_expiry_id
ON customers(contract_end_date, id);

CREATE INDEX idx_usage_customer_month
ON usage_history(customer_id, usage_month DESC);
```

Expected query latency:

- Search < 100ms
- Customer detail < 50ms

---

## Future Scale (10M+ Customers)

### Partition Usage History

```sql
PARTITION BY RANGE (usage_month)
```

Example:

- usage_history_2026_q1
- usage_history_2026_q2
- usage_history_2026_q3

Benefits:

- Faster scans
- Easier archival

---

### Read Replicas

```text
                +------------+
                | Primary DB |
                +-----+------+
                      |
        +-------------+-------------+
        |                           |
        v                           v
+---------------+         +---------------+
| Read Replica  |         | Read Replica  |
+---------------+         +---------------+
```

Read traffic:

- Customer search
- Customer detail pages

Write traffic:

- Pitch generation
- Bulk jobs

---

### Sharding (Future)

Potential shard key:

```text
customer_id
```

Example:

```text
Shard A: 1-10M
Shard B: 10M-20M
Shard C: 20M-30M
```

Only required beyond tens of millions of customers.

---

# Bulk Job Processing

## Workflow

```text
Agent
  |
  v
Create Bulk Job
  |
  v
bulk_jobs
  |
  v
bulk_job_items
  |
  v
Worker Pool
  |
  v
LLM Provider
```

---

## Concurrency Control

Worker count:

```text
10 workers
```

Batch size:

```text
100 customers
```

Benefits:

- Controlled database load
- Controlled LLM request volume

---

## Backpressure

Bounded queue:

```text
Maximum Queue Size: 1000 jobs
```

When exceeded:

```http
429 Too Many Requests
```

or

```http
202 Accepted
```

depending on system policy.

---

# Observability

## Metrics

### API

- Request count
- Request latency
- Error rate
- Active requests

### Database

- Query duration
- Connection pool usage
- Slow query count

### LLM

- Request count
- Failure rate
- Latency
- Token usage

### Jobs

- Queue depth
- Completed jobs
- Failed jobs
- Retry count

---

## Logging

Structured JSON logs.

Example:

```json
{
  "customer_id": 123,
  "job_id": "abc-123",
  "status": "SUCCESS",
  "latency_ms": 420
}
```

---

## Alerting

Alerts:

- API error rate > 5%
- Database latency > 500ms
- LLM failure rate > 10%
- Queue depth > 10,000
- Worker failure spikes

---

# High Availability

## API Layer

Multiple instances behind load balancer.

Benefits:

- No single point of failure
- Rolling deployments
- Horizontal scaling

---

## Database

Production setup:

```text
Primary PostgreSQL
    |
    +---- Replica #1
    +---- Replica #2
```

Benefits:

- Read scaling
- Disaster recovery

---

## Redis

Production setup:

```text
Redis Sentinel
```

or

```text
Managed Redis Service
```

for failover support.

---

# Deployment

## Docker Compose (Local)

Services:

- api
- postgres
- redis
- worker

```bash
docker compose up -d
```

---

## Production Deployment

Example:

- DigitalOcean Droplet
- AWS ECS
- Fly.io
- Railway

Deployment pipeline:

```text
GitHub
   |
   v
GitHub Actions
   |
   v
Docker Build
   |
   v
Container Registry
   |
   v
Production Deployment
```

---

# Security Considerations

- Secrets stored in environment variables
- Database credentials rotated regularly
- HTTPS enabled
- LLM API keys never exposed to frontend
- Input validation on all endpoints
- Rate limiting on pitch generation endpoints

---

# Disaster Recovery

Backups:

- Daily PostgreSQL backup
- Point-in-time recovery enabled

Recovery Objective:

- RPO: 15 minutes
- RTO: 1 hour

---

# Design Tradeoffs

| Decision | Choice | Reason |
|-----------|---------|---------|
| Database | PostgreSQL | Strong indexing and query performance |
| Cache | Redis | Fast lookup and rate limiting |
| Pagination | Cursor | Better performance at scale |
| Bulk Processing | Worker Pool | Prevent API timeout |
| Pitch Storage | Cached | Avoid duplicate LLM cost |
| Scaling | Read Replicas First | Simpler than sharding |
| Search | Indexed SQL Search | Sufficient for 500k rows |