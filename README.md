# Customer Retention Sales Platform

> [!NOTE]
> **Live Production Deployment**: [https://time-retention-service.onrender.com](https://time-retention-service.onrender.com)

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
3. Open the frontend dashboard at `http://localhost:8080` in your web browser.
4. Open the API documentation page (Swagger UI) at `http://localhost:8080/swagger` to inspect and execute API requests directly from localhost.
5. (Optional) To connect actual LLM endpoints rather than local mock templates, configure your API Key and target model in the `config.json` file, or via environment variables.

### Run the Unit Tests
Execute the Go test suite inside a clean, temporary Alpine-Go container:
```bash
docker run --rm -v "$(pwd)":/app -w /app golang:1.23-alpine sh -c "go mod tidy && go test -v ./..."
```

---

## 1.5 Configuration Management

The system loads settings using a multi-tiered configuration manager with the following order of precedence (highest to lowest):

1. **OS Environment Variables** (Highest precedence, typically set in `docker-compose.yml` or container hosts).
2. **`config.json`** (Local JSON configuration file located at the project root).
3. **`.env`** (Environment file containing local fallback configurations).
4. **Hardcoded Defaults** (e.g., `gemini-1.5-flash` model and default port `8080`).

### Configuration Schema (`config.json`)
```json
{
  "database_url": "postgres://postgres:postgres@localhost:5432/retention?sslmode=disable",
  "redis_url": "localhost:6379",
  "port": "8080",
  "gemini_api_key": "YOUR_GEMINI_API_KEY",
  "llm_model": "gemini-1.5-flash",
  "role": "both"
}
```

### Supported Parameters
- `database_url` / `DATABASE_URL`: Connection string for PostgreSQL.
- `redis_url` / `REDIS_URL`: Connection details for Redis.
- `port` / `PORT`: Port for the API server (default `8080`).
- `gemini_api_key` / `GEMINI_API_KEY`: Google Gemini API Key. If left empty, the application falls back to a template-based mock pitch generator.
- `llm_model` / `LLM_MODEL`: The Gemini model to target (e.g. `gemini-1.5-flash`, `gemini-1.5-pro`).
- `role` / `ROLE`: Node execution role (`api`, `worker`, or `both`).

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

### Infrastructure Components Details

#### 1. API Service
- **Responsibilities**: Customer search, details retrieval, creating individual pitch generation requests, creating bulk generation jobs, and job status retrieval.
- **Technology**: Go, Gin Framework, and the high-performance `pgx` PostgreSQL driver.
- **Scaling**: Fully stateless and horizontally scalable behind a load balancer.

#### 2. PostgreSQL (Primary DB & Replicas)
- **Stores**: Customers, usage history, cached pitches, bulk jobs, and bulk job items.
- **Role**: Strong consistency, mature indexing, partitioning, and read replicas support.

#### 3. Redis Cache & Rate Limiting
- **Pitch Cache**: Keyed on `pitch:{customer_id}:{hash}` to avoid regenerating pitches when customer data remains identical.
- **Rate Limiting**: Protects downstream LLM APIs from burst traffic.
- **Distributed Coordination**: Enables distributed locking mechanisms to avoid duplicate workers pulling the same campaign segments.

#### 4. Asynchronous Worker Pool
- **Responsibilities**: Processes asynchronous bulk pitch campaigns, implements exponential retry handling, and controls backpressure.
- **Benefits**: Prevents web timeouts, enforces rate limits, and protects downstream services.

#### 5. LLM Provider
- **Support**: Google Gemini API integration (built-in template mock generator is activated when no API key is supplied).
- **Prompt Structure**: Maps demographics, tenure, plans, and average monthly download/upload statistics into a personalized retention email structure.

---

## 2.5 Project Directory Structure

The repository is structured to separate concern and layer boundaries clearly:

```text
├── db/                          # Database migrations & schemas
│   ├── schema.sql               # PostgreSQL tables, indices, and extensions definitions
│   └── seed.sql                 # High-speed data seeding script (500k customers + 1.5M usage history records)
│
├── internal/                    # Private application & business logic
│   ├── config/                  # Multi-tiered configuration manager (JSON, Dotenv, and OS Env)
│   ├── db/                      # Database client connection, tuning, and bootstrap logic
│   ├── models/                  # Shared data transfer objects (DTOs) and DB mapping structs
│   ├── llm/                     # Google Gemini LLM API client wrapper & template mock fallback
│   ├── server/                  # HTTP controller actions, routes, and Gin server definitions
│   └── worker/                  # Background worker threads & transactional atomic coordinator
│
├── static/                      # Static web assets and API documents
│   ├── index.html               # Frontend single-page application dashboard
│   ├── style.css                # Visual layout (glassmorphism UI styles)
│   ├── app.js                   # Client-side reactivity, cursor-based sorting, and real-time polling
│   ├── swagger.yaml             # OpenAPI 3.0 REST API specification
│   └── swagger.html             # CDN-loaded Swagger UI documentation console
│
├── Dockerfile                   # Multi-stage production container build rules
├── docker-compose.yml           # Decentralized infrastructure local runner config
├── render.yaml                  # Production environment Blueprint specification (Render.com)
├── config.json                  # Local configuration keys and models options
├── .env                         # Standard local environment variables template
└── main.go                      # Application main entry point (starts server and worker processes)
```

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

## 4. Production & Infrastructure Considerations

### Keyset vs Offset Pagination
Offset pagination (`OFFSET 100000 LIMIT 15`) requires the database to read and discard all 100,000 records sequentially, leading to latency degradation at scale. We implemented **Keyset (Cursor) Pagination** leveraging a composite index `(contract_end_date, id)`. This allows PostgreSQL to jump directly to the target page via B-Tree lookup, ensuring **O(log N) constant latency** regardless of how deep the agent scrolls.

### Asynchronous Queue & Backpressure
Bulk generation requests run asynchronously in a worker pool.
- **Backpressure**: The API handles incoming bulk jobs via a bounded Go channel (`JobQueue`) with a limit of 1,000 active jobs. If exceeded, the API immediately throws `429 Too Many Requests` to protect the host's memory resources.
- **LLM Rate-Limiter**: Workers pull items and enforce a token bucket rate limit of **5 requests per second** to prevent hitting rate limits or triggering billing limits at the LLM provider side.
- **Fail-safe batching**: If an individual pitch generation fails (e.g., LLM timeout), only that customer's status is updated to `FAILED`. The worker captures the error message and increments `failed_count` on the job parent record, allowing agents to see successes and failures granularly inside the bulk jobs tracker.

### High Availability (HA) Setup
1. **API Layer**: Multiple instances placed behind a load balancer to ensure zero single point of failure and support rolling zero-downtime updates.
2. **Database Layer**: Production environment should use a primary node with multiple replication nodes (read-splitting reads off replicas, writes routed to the primary).
3. **Cache Layer**: Redis Sentinel or AWS ElastiCache cluster configuration to guarantee automatic failovers.

### Deployment Pipelines
1. **Local (Docker Compose)**: Compiles `api`, `worker`, `postgres`, and `redis` locally in isolated network structures using a standard `.env` configuration file.
2. **Production Pipeline (Render Free Tier)**: We support automated deployment to **Render.com's Free Tier** using the `render.yaml` Blueprint file. The live platform is deployed at [https://time-retention-service.onrender.com](https://time-retention-service.onrender.com).
   - **Infrastructure Mapping**: Automatically configures a Free Managed PostgreSQL database, a Free Managed Redis cache, and a unified Web Service running with `ROLE=both` (which executes the API router and background worker queue loops inside a single container instance to fit under Free Tier constraints).
   - **Deployment Steps**:
     1. Push your project repository to GitHub or GitLab.
     2. Open your Render Dashboard, navigate to **Blueprints**, and click **New Blueprint Instance**.
     3. Connect your repository.
     4. Render will parse `render.yaml` and prompt you for configuration details. Input your `GEMINI_API_KEY` (or leave it blank to default to mock pitches).
     5. Click **Deploy** to automatically configure, build, and deploy the entire environment.

---

### Security Enforcements
- Secrets and tokens loaded dynamically and masked in diagnostic outputs.
- Database access and execution configurations locked under TLS parameters.
- LLM API keys never exposed to client-side browsers.
- Input validation and rate-limiting blocks protect critical API controllers.

### Disaster Recovery Target
- **Daily backups**: Automatic snapshot exports of PostgreSQL primary volumes.
- **Point-in-time recovery (PITR)**: Write-Ahead Logs (WAL) archived to object stores (e.g., AWS S3).
- **RPO Target**: 15 minutes.
- **RTO Target**: 1 hour.

---

## 4.5 System Design Tradeoffs

| Decision | Choice | Reason |
|:---|:---|:---|
| **Database** | PostgreSQL | Strong relational consistency, robust indexing planner, range partitioning, and fuzzy trigram matching. |
| **Cache Store** | Redis | Fast O(1) read/write lookups, TTL auto-expiry, and rate-limiting bucket capability. |
| **Pagination** | Keyset / Cursor | Guarantees constant O(log N) query latency, avoiding database read fatigue caused by OFFSET scans. |
| **Bulk Engine** | Background Worker | Isolates intensive external API requests, controls backpressure, and prevents HTTP request timeouts. |
| **Pitch Caching** | Write-Through Caching | Minimizes operational LLM token expenses and reduces API invocation delays to sub-millisecond ranges. |
| **Scaling Route** | Read Replicas First | Distributing lookup queries to read replicas is significantly simpler and cheaper than early horizontal sharding. |
| **Search Engine** | GIN Trigram Index | Sufficiently performant (<10ms) for fuzzy wildcard matches up to 5M rows, avoiding Elasticsearch infrastructure overhead. |
---

## 5. Database Schema & Index Design

### Entity Relationship Diagram

```text
customers
    ├── 1:N ──> usage_history
    ├── 1:1 ──> generated_pitches
    ├── 1:N ──> bulk_job_items
    └── 1:N ──> pitch_generation_events

bulk_jobs
    └── 1:N ──> bulk_job_items
```

### Table Specifications

#### 1. `customers`
Stores customer master data. Used for customer listing, fuzzy search, and profile page details.
```sql
CREATE TABLE customers (
    id BIGSERIAL PRIMARY KEY,
    customer_number VARCHAR(50) UNIQUE NOT NULL,
    full_name TEXT NOT NULL,
    plan_name VARCHAR(100),
    contract_start_date DATE,
    contract_end_date DATE,
    monthly_fee NUMERIC(10,2),
    tenure_months INT,
    version INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### 2. `usage_history`
Stores monthly internet usage statistics (downloaded/uploaded gigabytes). Used to calculate customer averages during pitch generation.
```sql
CREATE TABLE usage_history (
    id BIGSERIAL PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    usage_month DATE NOT NULL,
    download_gb NUMERIC(10,2),
    upload_gb NUMERIC(10,2),
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### 3. `generated_pitches`
Caches the latest generated pitch for a customer based on a hash of their details. Prevents redundant LLM calls when details haven't changed.
```sql
CREATE TABLE generated_pitches (
    customer_id BIGINT PRIMARY KEY REFERENCES customers(id) ON DELETE CASCADE,
    customer_data_hash VARCHAR(64),
    pitch_text TEXT,
    llm_model VARCHAR(50),
    generated_at TIMESTAMP
);
```

#### 4. `bulk_jobs`
Represents an asynchronous bulk campaign request.
```sql
CREATE TABLE bulk_jobs (
    id UUID PRIMARY KEY,
    status VARCHAR(20),       -- 'PENDING', 'PROCESSING', 'COMPLETED', 'FAILED'
    total_count INT,
    completed_count INT,
    failed_count INT,
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### 5. `bulk_job_items`
Tracks progress for individual customers within a parent bulk job campaign.
```sql
CREATE TABLE bulk_job_items (
    id BIGSERIAL PRIMARY KEY,
    bulk_job_id UUID REFERENCES bulk_jobs(id) ON DELETE CASCADE,
    customer_id BIGINT REFERENCES customers(id) ON DELETE CASCADE,
    status VARCHAR(20),       -- 'PENDING', 'PROCESSING', 'SUCCESS', 'FAILED'
    error_message TEXT,
    processed_at TIMESTAMP
);
```

#### 6. `pitch_generation_events`
Stores operational audit logs for LLM latency, token counts, and cost/retry diagnostics.
```sql
CREATE TABLE pitch_generation_events (
    id BIGSERIAL PRIMARY KEY,
    customer_id BIGINT REFERENCES customers(id) ON DELETE CASCADE,
    status VARCHAR(20),       -- 'SUCCESS', 'FAILED'
    model_name VARCHAR(50),
    prompt_tokens INT,
    completion_tokens INT,
    latency_ms BIGINT,
    retry_count INT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### Database Index Configurations
- **`idx_customer_contract_end`**: Scans expiring contracts efficiently.
  ```sql
  CREATE INDEX idx_customer_contract_end ON customers(contract_end_date);
  ```
- **`idx_customer_expiry_id`**: Powers Keysets cursor pagination without offsets.
  ```sql
  CREATE INDEX idx_customer_expiry_id ON customers(contract_end_date, id);
  ```
- **`idx_usage_customer_month`**: Pulls chronological customer usage history.
  ```sql
  CREATE INDEX idx_usage_customer_month ON usage_history(customer_id, usage_month DESC);
  ```
- **`idx_bulk_job_items_job`**: Fetches bulk campaign item progress listings rapidly.
  ```sql
  CREATE INDEX idx_bulk_job_items_job ON bulk_job_items(bulk_job_id);
  ```
- **`idx_customer_name_trgm`**: Performs sub-millisecond name-based fuzzy wildcard matching (`full_name ILIKE '%john%'`).
  ```sql
  CREATE INDEX idx_customer_name_trgm ON customers USING gin(full_name gin_trgm_ops);
  ```

### Future Scaling Architecture
1. **Range Partitioning**: Partition the `usage_history` table by `usage_month` range to prevent indices from exceeding memory limits as data grows past 10M+ rows.
2. **Read/Write Splitting**: Direct query reads (customer listing, detail views, stats) to PostgreSQL Read Replicas, keeping the Primary node dedicated to writes.
3. **Horizontal Sharding**: Partition the customer catalog by `customer_id` hash ranges once traffic scales to tens of millions of active profiles.

---

## 6. REST API Reference

All requests and responses use the `application/json` format.

**Base URL path**: `/api/v1`

### Endpoints Map

| Endpoint | HTTP Method | Description |
|---|---|---|
| `/health` | `GET` | Service status healthcheck |
| `/customers` | `GET` | Query paginated customer list |
| `/customers/{id}` | `GET` | Query single customer details |
| `/customers/{id}/pitch` | `POST` | Generate or fetch cached recontract pitch |
| `/customers/{id}/pitch` | `GET` | Query previously cached pitch |
| `/bulk-pitches` | `POST` | Trigger bulk generation campaign |
| `/bulk-pitches` | `GET` | List history of bulk job campaigns |
| `/bulk-pitches/{id}` | `GET` | Query bulk campaign statistics |
| `/bulk-pitches/{id}/items` | `GET` | Query status of campaign items |

---

### Endpoint Specifications

#### 1. Service Health
* **URL**: `GET /health`
* **Response (200 OK)**:
  ```json
  {
    "status": "healthy"
  }
  ```

#### 2. List Customers
* **URL**: `GET /customers`
* **Query Parameters**:
  * `search` (string, optional): Search by customer name.
  * `plan` (string, optional): Filter by plan type.
  * `expires_before` (string, optional, YYYY-MM-DD): Filter by contract expiry date.
  * `cursor` (string, optional): Cursor pagination token.
  * `limit` (integer, optional, default 15): Page size.
* **Response (200 OK)**:
  ```json
  {
    "data": [
      {
        "id": 1,
        "name": "John Tan",
        "plan_name": "1Gbps",
        "contract_end_date": "2026-07-01"
      }
    ],
    "next_cursor": "2026-07-01T00:00:00Z|1"
  }
  ```

#### 3. Get Customer Details
* **URL**: `GET /customers/{customer_id}`
* **Response (200 OK)**:
  ```json
  {
    "id": 1,
    "customer_number": "CN-1002934",
    "name": "John Tan",
    "plan_name": "1Gbps",
    "monthly_fee": 59.99,
    "tenure_months": 24,
    "contract_start_date": "2024-07-01",
    "contract_end_date": "2026-07-01",
    "usage_history": [
      {
        "month": "2026-05",
        "download_gb": 540.23,
        "upload_gb": 120.45
      }
    ]
  }
  ```
* **Response (404 Not Found)**:
  ```json
  {
    "error": "not_found"
  }
  ```

#### 4. Generate Personalized Pitch
* **URL**: `POST /customers/{customer_id}/pitch`
* **Response (200 OK)**:
  ```json
  {
    "customer_id": 1,
    "pitch": "Hi John, thank you for being with us on our 1Gbps plan for 24 months...",
    "generated_at": "2026-06-22T02:40:00Z",
    "cached": false,
    "model": "gemini-1.5-flash"
  }
  ```

#### 5. Get Existing Pitch
* **URL**: `GET /customers/{customer_id}/pitch`
* **Response (200 OK)**:
  ```json
  {
    "customer_id": 1,
    "pitch": "Hi John, thank you for being with us...",
    "generated_at": "2026-06-22T02:40:00Z"
  }
  ```

#### 6. Create Bulk Pitch Job
* **URL**: `POST /bulk-pitches`
* **Request Body**:
  ```json
  {
    "filters": {
      "expires_before": "2026-07-31",
      "plan_name": "1Gbps"
    }
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "job_id": "a239a108-9dda-419c-acea-5c5ebcc98ed6",
    "status": "PENDING"
  }
  ```
* **Response (429 Too Many Requests - Backpressure)**:
  ```json
  {
    "error": "rate_limited",
    "message": "Too many active bulk jobs. Please try again later."
  }
  ```

#### 7. List Bulk Pitch Jobs
* **URL**: `GET /bulk-pitches`
* **Response (200 OK)**:
  ```json
  {
    "data": [
      {
        "job_id": "a239a108-9dda-419c-acea-5c5ebcc98ed6",
        "status": "COMPLETED",
        "total_count": 1000,
        "completed_count": 995,
        "failed_count": 5,
        "created_at": "2026-06-21T17:38:19Z"
      }
    ]
  }
  ```

#### 8. Get Bulk Job Status
* **URL**: `GET /bulk-pitches/{job_id}`
* **Response (200 OK)**:
  ```json
  {
    "job_id": "a239a108-9dda-419c-acea-5c5ebcc98ed6",
    "status": "PROCESSING",
    "total_count": 1000,
    "completed_count": 650,
    "failed_count": 10,
    "created_at": "2026-06-21T17:38:19Z"
  }
  ```

#### 9. Get Bulk Job Items
* **URL**: `GET /bulk-pitches/{job_id}/items`
* **Query Parameters**:
  * `status` (string, optional, values `PENDING`, `PROCESSING`, `SUCCESS`, `FAILED`): Filter results.
  * `cursor` (string, optional): Keyset pagination cursor token (item ID).
  * `limit` (integer, optional, default 50): Page size.
* **Response (200 OK)**:
  ```json
  {
    "data": [
      {
        "customer_id": 1,
        "status": "SUCCESS"
      },
      {
        "customer_id": 2,
        "status": "FAILED",
        "error_message": "LLM timeout"
      }
    ],
    "next_cursor": "2"
  }
  ```

---

### Core HTTP Response Errors

#### 400 Bad Request (Validation Error)
```json
{
  "error": "invalid_request",
  "message": "expires_before is invalid"
}
```

#### 404 Not Found
```json
{
  "error": "not_found"
}
```

#### 409 Conflict (Optimistic Locking Error)
```json
{
  "error": "conflict",
  "message": "customer was modified by another user"
}
```

#### 429 Too Many Requests
```json
{
  "error": "rate_limited"
}
```

---

## 7. Operational Observability

For production reliability and cost analytics, the platform supports monitoring across multiple metrics dimensions:

### 1. Key Metrics Dimensions
- **API Traffic**: Request volumes, route latencies, HTTP 4xx/5xx ratios, and active load parameters.
- **Database Health**: Active query runtimes, connection pool utilization, transaction lock durations, and slow-scans counts.
- **Downstream LLM**: API response latency tracking, query cost token counting (prompt and completion), and failure rates.
- **Job Engine**: Background queue depths, completed/failed item counts, and thread pool execution speeds.

### 2. Structured JSON Logging Schema
The API and worker services write logs in structured JSON format to standard output. Example:
```json
{
  "timestamp": "2026-06-22T02:40:00Z",
  "level": "INFO",
  "customer_id": 123,
  "job_id": "a239a108-9dda-419c-acea-5c5ebcc98ed6",
  "status": "SUCCESS",
  "latency_ms": 420
}
```

### 3. Prometheus Alerting Thresholds
Recommended production notification thresholds:
- Alert if **API HTTP 5xx Error Rate** exceeds **5%** within 5 minutes.
- Alert if **Database Query Execution Latency** exceeds **500ms** (identifies missing indices).
- Alert if **LLM Provider API Failure Rate** exceeds **10%** within 5 minutes (network blips or rate limits).
- Alert if **Background Work Queue Depth** exceeds **10,000 items** (signifies worker throughput depletion).

---

## 8. Strategic Concerns & Proposed Improvements

Below is an engineering outline of architectural trade-offs, constraints, and planned structural upgrades for the platform:

### 1. Concurrency & Data Integrity
* **Concern: Concurrent Customer Updates**: If two agents edit a customer profile (e.g., plan details or recontract date) simultaneously, data overrides can occur.
* **Current Hash Caching Protection**: The cache layers use SHA256 hashes of customer data. If two agents click "Generate Pitch" concurrently, the system recognizes identical hashes and pulls from cache directly, eliminating duplicate LLM expenses.
* **Proposed Upgrade (Optimistic Locking)**:
  Enforce optimistic concurrency control on edits:
  ```sql
  UPDATE customers 
  SET plan_name = $1, contract_end_date = $2, version = version + 1, updated_at = NOW()
  WHERE id = $3 AND version = $4;
  ```
  If 0 rows are affected, return `409 Conflict`, indicating another agent updated the customer profile. The agent's UI should refresh with the updated data.

### 2. Failure Handling & Reliability
* **Concern: Resiliency of Large Campaigns**: Bulk jobs can contain thousands of items. A downstream timeout or rate limit shouldn't fail the entire campaign.
* **Granular Item Tracking**: The system processes bulk items individually. Failures are written to the database with specific errors (`bulk_job_items.error_message`), allowing completed tasks to succeed.
* **Proposed Upgrades**:
  * **Selective Retry Action**: Add a UI action enabling agents to retry only the `FAILED` items within a bulk campaign, preventing the reprocessing of successful items.
  * **Exponential Backoff**: Implement jittered exponential backoffs on downstream LLM integrations to absorb transient network spikes automatically.

### 3. Scale & Infrastructure (500k to 10M+ rows)
* **Concern: Memory Limits of Go Channels**: Standard Go channels (`JobQueue` and `WorkQueue`) reside in memory. In-memory queues risk losing items if worker containers restart.
* **Proposed Upgrades**:
  * **Persistent Message Broker**: Shift task distribution from Go memory channels to a persistent message broker (e.g., **RabbitMQ** or **Apache Kafka**).
  * **Distributed Locking (Redis Redlock)**: Enforce Redlock rules to ensure coordinator nodes do not collide when processing bulk jobs across multiple replica containers.
  * **Dedicated Message Broker vs. Database Polling**:
    * **The Decision**: For scale, we outline using a dedicated message broker (e.g., RabbitMQ or Amazon SQS) to manage the work queue rather than having the worker pool continuously poll the PostgreSQL database (e.g., executing `SELECT * FROM bulk_job_items WHERE status = 'PENDING' FOR UPDATE SKIP LOCKED`).
    * **The Why**: Using a database as a high-throughput queue is a notorious scaling anti-pattern. If hundreds of workers continuously poll the database, it creates severe CPU contention, exhausts connection limits, and requires row-locking overhead. A dedicated message broker natively handles concurrency, visibility timeouts, and scales infinitely without taxing the core database. It also simplifies operations for retrying failed generation tasks (re-ops).
    * **The Trade-off**: Introduces an additional piece of infrastructure to provision, monitor, and pay for, as well as eventual consistency state orchestration concerns between the message broker and PostgreSQL.
  * **Publishing Batches to Message Queue vs. Individual Tasks (Cost vs. Granularity)**:
    * **The Decision**: For bulk campaigns, publish task batches (e.g., 1 batch containing 100 customer items) to the message queue rather than individual messages.
    * **The Why**: Emitting 100,000 individual messages to the broker could result in API throttling and high per-request message fees. Batching dramatically increases enqueuing throughput and lowers queue system utilization.
    * **The Trade-off**: We sacrifice queue-native granular failure isolation. If a worker picks up a batch of 100 customer profiles and pitch generation #99 fails, the worker must implement transactional logic to write the 99 successes as `SUCCESS` to the database and individually mark the 1 failure as `FAILED` in the `bulk_job_items` table, rather than relying on automatic message-level retries. We accepted this worker complexity to gain cost savings and throughput efficiency.

### 4. Observability & Auditing
* **Concern: LLM Token Cost Spikes**: Real-time LLM integration can escalate costs rapidly without operational visibility.
* **Audit Logging**: The platform writes detailed latency and cost structures (prompt + completion tokens) to `pitch_generation_events`.
* **Proposed Upgrades**:
  * **Prometheus Metrics**: Expose Prometheus endpoints (`/metrics`) to monitor Redis cache hit ratios, LLM latencies, and queue depths.
  * **Budget Alerts**: Set up automatic Slack/PagerDuty warnings if the hourly LLM token expenditure exceeds budgeted thresholds.
