# Concerns and Improvements

This document outlines structural design trade-offs, concerns, and proposed improvements for the Retention Agent Platform, categorized by layer.

---

## 1. Concurrency & Data Integrity

### Concern: Concurrent Updates by Multiple Agents
* **Context**: The `assignment.md` prompts us to address what happens when two agents act on the same customer at once.
* **Current Implementation**:
  - We have a `version` column in the `customers` table.
  - When generating pitches, the system calculates a SHA256 hash of the customer's data and usage history. If two agents click "Generate Pitch" at the same time, the hash is identical, triggering cache hits (reducing LLM costs).
* **Proposed Improvement (Optimistic Locking)**:
  - When an agent edits a customer profile (e.g., updating a plan or contract date), the backend should enforce optimistic locking:
    ```sql
    UPDATE customers 
    SET plan_name = $1, contract_end_date = $2, version = version + 1, updated_at = NOW()
    WHERE id = $3 AND version = $4;
    ```
  - If this query affects 0 rows, the backend must return a `409 Conflict` error (defined in `api.md`), indicating that another agent has modified the record in the meantime. The agent's UI should refresh and display the updated details.

---

## 2. Failure Handling & Reliability

### Concern: Resiliency of Bulk Pitch Processing
* **Context**: A bulk pitch job can encompass thousands of customers. If a network blip occurs or the LLM provider rate limits us, the entire batch should not fail.
* **Current Implementation**:
  - **Granular Item Status**: Individual customers are processed as independent `bulk_job_items`. If one fails, it is marked as `FAILED` with an error message (e.g., `"LLM timeout"`), while others continue processing.
  - **Progress Visibility**: The `bulk_jobs` parent record keeps track of `completed_count` and `failed_count` in real-time.
* **Proposed Improvements**:
  - **Selective Retry Action**: Add a button on the UI next to failed job items to let agents retry only the failed customers in that batch, instead of running the whole filter campaign again.
  - **Exponential Backoff**: Implement exponential backoff in workers when contacting the external LLM API to gracefully handle transient network errors.

---

## 3. Scale & Infrastructure (500k to 10M+ Customers)

### Concern: Memory & Network Limits in Go Channels
* **Context**: We use bounded Go channels (`JobQueue` and `WorkQueue`) for job coordination.
* **Current Design**:
  - `JobQueue` is capped at 1,000 jobs (backpressure triggers `429 Too Many Requests` when full).
  - `WorkQueue` holds up to 10,000 pending customer items.
* **Proposed Improvements**:
  - **Distributed Message Broker**: As the system scales to 10M+ rows, Go memory channels should be replaced by a distributed broker like **RabbitMQ** or **Apache Kafka**. This guarantees message persistence if worker containers crash mid-job.
  - **Distributed Locking (Redis Redlock)**: Use Redis distributed locks to ensure that only one worker instance coordinates a specific bulk job, avoiding race conditions during batch state transitions.

### Concern: Database Bottlenecks
* **Context**: Scanning 10M+ rows or bulk-inserting items can degrade database write latency.
* **Current Design**:
  - Highly optimized SQL query that inserts all `bulk_job_items` in a single transaction directly from a subquery:
    ```sql
    INSERT INTO bulk_job_items (bulk_job_id, customer_id, status)
    SELECT $1, id, 'PENDING' FROM customers WHERE ...
    ```
  - Keyset pagination indexes on `(contract_end_date, id)`.
* **Proposed Improvements**:
  - **PostgreSQL Range Partitioning**: Partition the `usage_history` table by `usage_month` (e.g., quarterly partitions). Since usage history is time-series data, partitioning ensures indexes stay memory-resident, speeding up queries.
  - **Read/Write Splitting**: Direct all read-heavy queries (e.g., customer search and detail views) to read replicas, leaving the primary database dedicated to updates, caching, and job state writes.

---

## 4. Observability & Auditing

### Concern: LLM Token Costs & Performance Drift
* **Context**: High-scale LLM integrations can accumulate significant API costs and hit latency bottlenecks.
* **Current Design**:
  - We log every generation request in the `pitch_generation_events` table (prompt tokens, completion tokens, latency, status, error).
* **Proposed Improvements**:
  - **Prometheus Metrics**: Expose Prometheus endpoints (`/metrics`) to track:
    - `redis_cache_hit_ratio`: Hit rate of pitch cache.
    - `llm_latency_seconds`: Histogram of LLM response latencies.
    - `worker_queue_depth`: Current depth of the work queue.
  - **Cost Alerting**: Set up alerts in Grafana/CloudWatch if the average token cost per hour spikes above a specific budget limit.
