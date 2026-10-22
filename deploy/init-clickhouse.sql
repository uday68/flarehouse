CREATE DATABASE IF NOT EXISTS flarehouse;

-- Raw Edge HTTP Telemetry Logs (High-throughput Ingestion)
CREATE TABLE IF NOT EXISTS flarehouse.edge_http_logs (
    timestamp DateTime64(3, 'UTC') DEFAULT now64(3),
    tenant_id LowCardinality(String),
    request_id UUID,
    client_ip String,
    host String,
    path String,
    method LowCardinality(String),
    status_code UInt16,
    response_bytes UInt64,
    duration_ms Float32,
    edge_colo LowCardinality(String),
    country LowCardinality(FixedString(2)),
    cache_status LowCardinality(String),
    threat_score UInt8 DEFAULT 0
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, toDate(timestamp), status_code, host, timestamp)
SETTINGS index_granularity = 8192;

-- Materialized View for 1-Minute Aggregated Analytics (Real-Time Rollup)
CREATE TABLE IF NOT EXISTS flarehouse.edge_metrics_1m (
    minute DateTime,
    tenant_id LowCardinality(String),
    edge_colo LowCardinality(String),
    status_code UInt16,
    request_count UInt64,
    total_bytes UInt64,
    total_duration_ms Float64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(minute)
ORDER BY (tenant_id, minute, edge_colo, status_code);

CREATE MATERIALIZED VIEW IF NOT EXISTS flarehouse.edge_metrics_1m_mv TO flarehouse.edge_metrics_1m AS
SELECT
    toStartOfMinute(timestamp) AS minute,
    tenant_id,
    edge_colo,
    status_code,
    count() AS request_count,
    sum(response_bytes) AS total_bytes,
    sum(duration_ms) AS total_duration_ms
FROM flarehouse.edge_http_logs
GROUP BY minute, tenant_id, edge_colo, status_code;

-- Unoptimized table deliberately created for benchmarking & AI Query Doctor demonstration
CREATE TABLE IF NOT EXISTS flarehouse.unoptimized_legacy_logs (
    timestamp DateTime,
    tenant_id String,
    client_ip String,
    url_blob String,
    raw_payload String
) ENGINE = MergeTree()
PARTITION BY toDate(timestamp) -- Daily partitioning anti-pattern causing too many small parts
ORDER BY tuple(); -- No sort key anti-pattern causing full table scans
