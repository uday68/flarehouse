-- ==============================================================================
-- FlareHouse Cloud Data Lake & Storage Tiering DDL
-- Connects ClickHouse OLAP with Google Cloud Storage (GCS) / Cloudflare R2
-- ==============================================================================

-- 1. Apply Tiered Storage Policy to Edge Telemetry Logs
-- Parts remain on fast local NVMe for 14 days, then automatically migrate to GCS Data Lake
ALTER TABLE flarehouse.edge_http_logs
    MODIFY SETTING storage_policy = 'tiered_data_lake_policy';

ALTER TABLE flarehouse.edge_http_logs
    MODIFY TTL timestamp + INTERVAL 14 DAY TO VOLUME 'cold_lake_volume';

-- 2. External Data Lake Export (Parquet format to GCP GCS for BigQuery & Spark Federation)
-- Allows cross-functional business intelligence and data science teams to query historical data in Parquet
/*
INSERT INTO FUNCTION s3(
    'https://storage.googleapis.com/cloudflare-centralized-data-lake/exports/edge_http_202610.parquet',
    '${GCP_HMAC_ACCESS_KEY}',
    '${GCP_HMAC_SECRET_KEY}',
    'Parquet'
)
SELECT 
    toStartOfHour(timestamp) as hour,
    tenant_id,
    edge_colo,
    status_code,
    country,
    count() as request_count,
    avg(duration_ms) as avg_latency_ms,
    sum(response_bytes) as total_egress_bytes
FROM flarehouse.edge_http_logs
WHERE toYYYYMM(timestamp) = 202610
GROUP BY hour, tenant_id, edge_colo, status_code, country;
*/
