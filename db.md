# Database Design

## Overview

The Retention Agent Platform is designed to support:

- 500,000+ customers
- Fast customer search and filtering
- Contract expiry tracking
- Customer usage analytics
- LLM-generated recontract pitches
- Bulk pitch generation
- Future scaling to millions of customers

The database is optimized for read-heavy workloads while maintaining efficient write performance for pitch generation and job processing.

---

# Entity Relationship Diagram

```text
customers
    |
    | 1:N
    |
usage_history

customers
    |
    | 1:1
    |
generated_pitches

customers
    |
    | 1:N
    |
bulk_job_items
    |
    | N:1
    |
bulk_jobs

customers
    |
    | 1:N
    |
pitch_generation_events
```

---

# Tables

## customers

Stores customer master data.

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

### Purpose

Used by:

- Customer listing
- Search
- Filtering
- Sorting
- Customer profile pages

---

## usage_history

Stores monthly internet usage statistics.

```sql
CREATE TABLE usage_history (
    id BIGSERIAL PRIMARY KEY,

    customer_id BIGINT NOT NULL
        REFERENCES customers(id),

    usage_month DATE NOT NULL,

    download_gb NUMERIC(10,2),
    upload_gb NUMERIC(10,2),

    created_at TIMESTAMP DEFAULT NOW()
);
```

### Purpose

Used for:

- Usage trend analysis
- Personalized pitch generation
- Customer profile view

---

## generated_pitches

Stores the latest generated pitch for a customer.

```sql
CREATE TABLE generated_pitches (
    customer_id BIGINT PRIMARY KEY
        REFERENCES customers(id),

    customer_data_hash VARCHAR(64),

    pitch_text TEXT,

    llm_model VARCHAR(50),

    generated_at TIMESTAMP
);
```

### Purpose

Used to:

- Cache generated pitches
- Avoid unnecessary LLM calls
- Reduce operational costs

---

## bulk_jobs

Represents a bulk pitch generation request.

```sql
CREATE TABLE bulk_jobs (
    id UUID PRIMARY KEY,

    status VARCHAR(20),

    total_count INT,
    completed_count INT,
    failed_count INT,

    created_at TIMESTAMP DEFAULT NOW()
);
```

### Status Values

- PENDING
- PROCESSING
- COMPLETED
- FAILED

---

## bulk_job_items

Tracks individual customer processing within a bulk job.

```sql
CREATE TABLE bulk_job_items (
    id BIGSERIAL PRIMARY KEY,

    bulk_job_id UUID
        REFERENCES bulk_jobs(id),

    customer_id BIGINT
        REFERENCES customers(id),

    status VARCHAR(20),

    error_message TEXT,

    processed_at TIMESTAMP
);
```

### Purpose

Provides visibility into:

- Successes
- Failures
- Retries

without failing the entire batch.

---

## pitch_generation_events

Stores audit and operational history.

```sql
CREATE TABLE pitch_generation_events (
    id BIGSERIAL PRIMARY KEY,

    customer_id BIGINT
        REFERENCES customers(id),

    status VARCHAR(20),

    model_name VARCHAR(50),

    prompt_tokens INT,
    completion_tokens INT,

    latency_ms BIGINT,

    retry_count INT,

    error_message TEXT,

    created_at TIMESTAMP DEFAULT NOW()
);
```

### Purpose

Supports:

- Monitoring
- Cost tracking
- Debugging
- Auditing

---

# Index Design

## idx_customer_contract_end

```sql
CREATE INDEX idx_customer_contract_end
ON customers(contract_end_date);
```

### Used By

```sql
SELECT *
FROM customers
WHERE contract_end_date <= CURRENT_DATE + INTERVAL '30 days';
```

### Why

Allows PostgreSQL to quickly locate expiring contracts without scanning the entire table.

---

## idx_customer_expiry_id

```sql
CREATE INDEX idx_customer_expiry_id
ON customers(contract_end_date, id);
```

### Used By

Cursor pagination queries.

```sql
SELECT *
FROM customers
WHERE (contract_end_date, id) >
      ($expiry_date, $customer_id)
ORDER BY contract_end_date, id
LIMIT 50;
```

### Why

Supports efficient keyset pagination.

Avoids expensive OFFSET scans.

---

## idx_usage_customer_month

```sql
CREATE INDEX idx_usage_customer_month
ON usage_history(customer_id, usage_month DESC);
```

### Used By

```sql
SELECT *
FROM usage_history
WHERE customer_id = $1
ORDER BY usage_month DESC;
```

### Why

Retrieves recent customer usage history efficiently.

---

## idx_bulk_job_items_job

```sql
CREATE INDEX idx_bulk_job_items_job
ON bulk_job_items(bulk_job_id);
```

### Used By

```sql
SELECT *
FROM bulk_job_items
WHERE bulk_job_id = $1;
```

### Why

Enables fast retrieval of job progress.

---

## Trigram Search Index

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_customer_name_trgm
ON customers
USING gin(full_name gin_trgm_ops);
```

### Used By

```sql
SELECT *
FROM customers
WHERE full_name ILIKE '%john%';
```

### Why

Supports fast fuzzy search on customer names.

Without this index PostgreSQL performs a sequential scan.

---

# Pagination Strategy

## Why Not OFFSET?

Example:

```sql
SELECT *
FROM customers
ORDER BY contract_end_date
LIMIT 50 OFFSET 100000;
```

Problem:

- Database must scan and discard 100,000 rows
- Performance degrades as dataset grows

---

## Cursor Pagination

Example:

```sql
SELECT *
FROM customers
WHERE (contract_end_date, id) >
      ($last_expiry, $last_id)
ORDER BY contract_end_date, id
LIMIT 50;
```

Benefits:

- Constant performance
- Index-friendly
- Suitable for millions of rows

---

# Future Scaling

## Usage History Partitioning

```sql
PARTITION BY RANGE (usage_month);
```

Benefits:

- Smaller indexes
- Faster scans
- Easier archival

---

## Read Replicas

Read-heavy operations can be moved to replicas:

- Customer search
- Customer details
- Reporting

Write operations remain on primary database.

---

## Sharding

Potential shard key:

```text
customer_id
```

Only required beyond tens of millions of customers.