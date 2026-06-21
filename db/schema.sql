CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS customers (
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

CREATE TABLE IF NOT EXISTS usage_history (
    id BIGSERIAL PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    usage_month DATE NOT NULL,
    download_gb NUMERIC(10,2),
    upload_gb NUMERIC(10,2),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS generated_pitches (
    customer_id BIGINT PRIMARY KEY REFERENCES customers(id) ON DELETE CASCADE,
    customer_data_hash VARCHAR(64),
    pitch_text TEXT,
    llm_model VARCHAR(50),
    generated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS bulk_jobs (
    id UUID PRIMARY KEY,
    status VARCHAR(20),
    total_count INT,
    completed_count INT,
    failed_count INT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bulk_job_items (
    id BIGSERIAL PRIMARY KEY,
    bulk_job_id UUID REFERENCES bulk_jobs(id) ON DELETE CASCADE,
    customer_id BIGINT REFERENCES customers(id) ON DELETE CASCADE,
    status VARCHAR(20),
    error_message TEXT,
    processed_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pitch_generation_events (
    id BIGSERIAL PRIMARY KEY,
    customer_id BIGINT REFERENCES customers(id) ON DELETE CASCADE,
    status VARCHAR(20),
    model_name VARCHAR(50),
    prompt_tokens INT,
    completion_tokens INT,
    latency_ms BIGINT,
    retry_count INT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indices
CREATE INDEX IF NOT EXISTS idx_customer_contract_end ON customers(contract_end_date);
CREATE INDEX IF NOT EXISTS idx_customer_expiry_id ON customers(contract_end_date, id);
CREATE INDEX IF NOT EXISTS idx_usage_customer_month ON usage_history(customer_id, usage_month DESC);
CREATE INDEX IF NOT EXISTS idx_bulk_job_items_job ON bulk_job_items(bulk_job_id);
CREATE INDEX IF NOT EXISTS idx_customer_name_trgm ON customers USING gin(full_name gin_trgm_ops);
