# Customer Retention Sales Platform

An internal customer retention portal built for scale to help retention agents browse expiring contracts, query details, view usage history trends, and generate personalized recontract sales pitches using LLMs.

---

## 1. Quick Start

### Prerequisites
- Docker & Docker Compose

### Run the System
1. From the project root, run:
   ```bash
   docker compose up -d --build
   ```
2. Wait a few seconds for database startup and bootstrap. The system will automatically seed **500,000 customers** and **1,500,000 usage history records** on first-time boot. Seeding takes ~15–20 seconds.
3. Open `http://localhost:8080` in your web browser.
4. (Optional) To connect actual LLM endpoints rather than local mock templates, provide your `GEMINI_API_KEY` in the `docker-compose.yml` environment variables.

---

## 2. Architecture & Components

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
                     +------------+------------+
                     |                         |
                     v                         v
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
                 v                              v
       +-------------------+          +-------------------+
       | PostgreSQL Read   |          |    LLM Provider   |
       |     Replica       |          | OpenAI / Gemini   |
       +-------------------+          +-------------------+
```

- **Stateless API Instances**: Built in Go using the **Gin** HTTP framework. Horizontally scalable.
- **Asynchronous Worker Pool**: Decoupled Go workers running on separate thread loops to handle bulk jobs without exhausting API resources.
- **Write-Through Cache Layer**:
  1. Computes a SHA256 signature hash of all relevant customer data (plan, tenure, fee, usage history).
  2. Checks Redis cache `pitch:{customer_id}:{hash}` first.
  3. If missed, checks PostgreSQL `generated_pitches` table cache.
  4. If missed, calls the LLM provider, updates both cache stores, and logs token usage/cost.
- **Database Layer**: PostgreSQL 16. Implements custom keyset indexes and fuzzy name search trigram indices.

---

## 3. High-Performance Index Optimizations

We seeded **500,000 customers** with **1,500,000 usage records** to validate performance. Below are the actual execution plan metrics from running `EXPLAIN ANALYZE` inside PostgreSQL.

### Query 1: Keyset Pagination Search
Retrieving customers sorted by contract end date and ID:
```sql
SELECT id, full_name, plan_name, contract_end_date 
FROM customers 
WHERE (contract_end_date, id) > ('2026-07-01', 45)
ORDER BY contract_end_date ASC, id ASC
LIMIT 15;
```

#### Before Optimisation (Without Index)
PostgreSQL performs a parallel sequential scan across all 500,000 rows, runs a heap sort, and discards all but the first 15:
```text
 Limit  (cost=13662.62..13664.37 rows=15 width=31) (actual time=29.342..31.086 rows=15 loops=1)
   ->  Gather Merge  (cost=13662.62..37797.97 rows=206860 width=31) (actual time=29.341..31.083 rows=15 loops=1)
         Workers Planned: 2
         Workers Launched: 2
         ->  Sort  (cost=12662.60..12921.17 rows=103430 width=31) (actual time=27.775..27.776 rows=15 loops=3)
               Sort Key: contract_end_date, id
               Sort Method: top-N heapsort  Memory: 26kB
               ->  Parallel Seq Scan on customers  (cost=0.00..10125.00 rows=103430 width=31)
                     Filter: (ROW(contract_end_date, id) > ROW('2026-07-01'::date, 45))
 Planning Time: 0.268 ms
 Execution Time: 31.151 ms
```

#### After Optimisation (With Index `idx_customer_expiry_id`)
We added a composite index:
```sql
CREATE INDEX idx_customer_expiry_id ON customers(contract_end_date, id);
```
Postgres scans only the next 15 records in the B-Tree directly, taking virtually no CPU:
```text
 Limit  (cost=0.42..2.59 rows=15 width=31) (actual time=0.074..0.079 rows=15 loops=1)
   ->  Index Scan using idx_customer_expiry_id on customers  (cost=0.42..35791.59 rows=248233 width=31) (actual time=0.074..0.079 rows=15 loops=1)
         Index Cond: (ROW(contract_end_date, id) > ROW('2026-07-01'::date, 45))
 Planning Time: 0.594 ms
 Execution Time: 0.125 ms
```
🚀 **Performance speedup**: **249x faster**

---

### Query 2: Fuzzy Name ILIKE Search
Filtering customers containing specific character patterns:
```sql
SELECT id, full_name, plan_name, contract_end_date 
FROM customers 
WHERE full_name ILIKE '%zach%'
LIMIT 15;
```

#### Before Optimisation (Without Index)
Postgres parallel-scans the full table to filter names matching the wildcard, taking ~58ms:
```text
 Limit  (cost=1000.00..4002.80 rows=15 width=31) (actual time=56.259..57.927 rows=0 loops=1)
   ->  Gather  (cost=1000.00..10608.97 rows=48 width=31) (actual time=56.259..57.926 rows=0 loops=1)
         Workers Planned: 2
         Workers Launched: 2
         ->  Parallel Seq Scan on customers  (cost=0.00..9604.17 rows=20 width=31) (actual time=54.636..54.637 rows=0 loops=3)
               Filter: (full_name ~~* '%zach%'::text)
 Planning Time: 0.252 ms
 Execution Time: 57.950 ms
```

#### After Optimisation (With Index `idx_customer_name_trgm`)
We enabled the `pg_trgm` extension and created a GIN (Generalized Inverted Index) trigram index:
```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_customer_name_trgm ON customers USING gin(full_name gin_trgm_ops);
```
PostgreSQL matches the character trigrams inside the index directly:
```text
 Limit  (cost=79.46..135.92 rows=15 width=31) (actual time=0.053..0.053 rows=0 loops=1)
   ->  Bitmap Heap Scan on customers  (cost=79.46..260.14 rows=48 width=31) (actual time=0.052..0.053 rows=0 loops=1)
         Recheck Cond: (full_name ~~* '%zach%'::text)
         ->  Bitmap Index Scan on idx_customer_name_trgm  (cost=0.00..79.45 rows=48 width=0) (actual time=0.049..0.049 rows=0 loops=1)
               Index Cond: (full_name ~~* '%zach%'::text)
 Planning Time: 0.571 ms
 Execution Time: 0.149 ms
```
🚀 **Performance speedup**: **388x faster**

---

## 4. Production Considerations

### Keyset vs Offset Pagination
Offset pagination (`OFFSET 100000 LIMIT 15`) requires the database to read and discard all 100,000 records sequentially, leading to latency degradation at scale. We implemented **Keyset (Cursor) Pagination** leveraging a composite index `(contract_end_date, id)`. This allows PostgreSQL to jump directly to the target page via B-Tree lookup, ensuring **O(log N) constant latency** regardless of how deep the agent scrolls.

### Asynchronous Queue & Backpressure
Bulk generation requests run asynchronously in a worker pool.
- **Backpressure**: The API handles incoming bulk jobs via a bounded Go channel (`JobQueue`) with a limit of 1,000 active jobs. If exceeded, the API immediately throws `429 Too Many Requests` to protect the host's memory resources.
- **LLM Rate-Limiter**: Workers pull items and enforce a token bucket rate limit of **5 requests per second** to prevent hitting rate limits or triggering billing limits at the LLM provider side.
- **Fail-safe batching**: If an individual pitch generation fails (e.g., LLM timeout), only that customer's status is updated to `FAILED`. The worker captures the error message and increments `failed_count` on the job parent record, allowing agents to see successes and failures granularly inside the bulk jobs tracker.
