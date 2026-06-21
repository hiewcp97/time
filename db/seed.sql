-- Seed 500,000 customers with realistic data
INSERT INTO customers (customer_number, full_name, plan_name, contract_start_date, contract_end_date, monthly_fee, tenure_months)
SELECT 
    'CUST' || LPAD(i::text, 8, '0'),
    (ARRAY['John', 'Jane', 'David', 'Sarah', 'Michael', 'Emily', 'Robert', 'Jessica', 'William', 'Ashley', 'James', 'Amanda', 'Charles', 'Mary', 'Joseph', 'Patricia', 'Daniel', 'Jennifer', 'Matthew', 'Linda'])[mod(i, 20) + 1] || ' ' ||
    (ARRAY['Smith', 'Johnson', 'Williams', 'Brown', 'Jones', 'Garcia', 'Miller', 'Davis', 'Rodriguez', 'Martinez', 'Hernandez', 'Lopez', 'Gonzalez', 'Wilson', 'Anderson', 'Thomas', 'Taylor', 'Moore', 'Jackson', 'Martin', 'Tan', 'Lim', 'Lee', 'Wong'])[mod(i, 24) + 1],
    (ARRAY['1Gbps', '500Mbps', '100Mbps', '2Gbps'])[mod(i, 4) + 1],
    (CURRENT_DATE - (mod(i, 24) + 12 || ' months')::interval)::date,
    (CURRENT_DATE + (mod(i, 6) - 2 || ' months')::interval)::date,
    (ARRAY[99.00, 129.00, 199.00, 299.00])[mod(i, 4) + 1],
    mod(i, 48) + 12
FROM generate_series(1, 500000) s(i)
ON CONFLICT (customer_number) DO NOTHING;

-- Seed usage history (3 months per customer)
INSERT INTO usage_history (customer_id, usage_month, download_gb, upload_gb)
SELECT 
    c.id,
    m.usage_month::date,
    (50 + mod(c.id + extract(month from m.usage_month)::int * 17, 900))::numeric(10,2),
    (5 + mod(c.id + extract(month from m.usage_month)::int * 7, 90))::numeric(10,2)
FROM customers c
CROSS JOIN (
    SELECT (date_trunc('month', CURRENT_DATE) - (s.i || ' month')::interval)::date AS usage_month
    FROM generate_series(0, 2) s(i)
) m
ON CONFLICT DO NOTHING;
