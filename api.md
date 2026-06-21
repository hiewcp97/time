# API Design

## Overview

The API supports:

- Customer browsing
- Search
- Filtering
- Sorting
- Pagination
- Individual pitch generation
- Bulk pitch generation
- Job monitoring

Base URL:

```text
/api/v1
```

---

# Customer APIs

## List Customers

```http
GET /customers
```

### Query Parameters

| Parameter | Description |
|------------|-------------|
| search | Customer name search |
| plan | Filter by plan |
| expires_before | Contract expiry filter |
| cursor | Cursor pagination token |
| limit | Page size |

### Example

```http
GET /customers?search=john&plan=1Gbps&limit=50
```

### Response

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
  "next_cursor": "..."
}
```

---

## Get Customer Details

```http
GET /customers/{customer_id}
```

### Response

```json
{
  "id": 1,
  "name": "John Tan",
  "plan_name": "1Gbps",
  "monthly_fee": 199,
  "tenure_months": 48,
  "contract_end_date": "2026-07-01",
  "usage_history": [
    {
      "month": "2026-05",
      "download_gb": 750
    }
  ]
}
```

---

# Pitch APIs

## Generate Customer Pitch

```http
POST /customers/{customer_id}/pitch
```

### Request

```json
{}
```

### Response

```json
{
  "customer_id": 1,
  "pitch": "Hi John, thank you for being with us...",
  "generated_at": "2026-06-21T12:00:00Z",
  "cached": false
}
```

---

## Get Existing Pitch

```http
GET /customers/{customer_id}/pitch
```

### Response

```json
{
  "customer_id": 1,
  "pitch": "Hi John...",
  "generated_at": "2026-06-21T12:00:00Z"
}
```

---

# Bulk Pitch APIs

## Create Bulk Job

```http
POST /bulk-pitches
```

### Request

```json
{
  "filters": {
    "expires_before": "2026-07-31",
    "plan_name": "1Gbps"
  }
}
```

### Response

```json
{
  "job_id": "f5f92a75-2b77-4ebd-94d4-c3fd47ea4c0",
  "status": "PENDING"
}
```

### Notes

Creates a background job.

Returns immediately.

---

## Get Job Status

```http
GET /bulk-pitches/{job_id}
```

### Response

```json
{
  "job_id": "f5f92a75-2b77-4ebd-94d4-c3fd47ea4c0",
  "status": "PROCESSING",
  "total_count": 1000,
  "completed_count": 650,
  "failed_count": 10
}
```

---

## Get Job Items

```http
GET /bulk-pitches/{job_id}/items
```

### Query Parameters

| Parameter | Description |
|------------|-------------|
| status | SUCCESS / FAILED |
| cursor | Cursor pagination |
| limit | Page size |

### Response

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
  ]
}
```

---

# Health Check

## Service Health

```http
GET /health
```

### Response

```json
{
  "status": "healthy"
}
```

---

# Error Handling

## Validation Error

```http
400 Bad Request
```

```json
{
  "error": "invalid_request",
  "message": "expires_before is invalid"
}
```

---

## Resource Not Found

```http
404 Not Found
```

```json
{
  "error": "not_found"
}
```

---

## Conflict

```http
409 Conflict
```

Returned when optimistic locking detects concurrent updates.

```json
{
  "error": "conflict",
  "message": "customer was modified by another user"
}
```

---

## Rate Limit

```http
429 Too Many Requests
```

```json
{
  "error": "rate_limited"
}
```

---

# API Design Decisions

## Asynchronous Bulk Processing

Bulk pitch generation uses background workers.

Benefits:

- Prevents API timeouts
- Supports large batches
- Enables retries

---

## Cursor Pagination

Used for:

- Customer listing
- Job item listing

Benefits:

- Consistent performance
- Scales to millions of records

---

## Cached Pitch Generation

Before generating a pitch:

1. Compute customer data hash
2. Compare with stored hash
3. Return cached pitch if unchanged

Benefits:

- Lower LLM costs
- Faster response times
- Reduced external dependency load