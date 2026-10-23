# FlareHouse Cold Partition Batch Rollup (PySpark)

This subproject provides a lightweight batch processing job that reads Parquet exports of cold ClickHouse partitions (e.g., archived to object storage such as GCS/S3 or local volume mounts) and aggregates granular telemetry into consolidated hourly rollups.

> **Scope & Realistic Expectations:**
> This job is designed for localized batch partition processing, periodic offline consolidation, and analytical verification on a single developer workstation. It does **not** claim or benchmark distributed cluster-scale results.

---

## 🎯 Architecture Overview

When ClickHouse migrates older partitions (e.g. $> 14$ days) from NVMe to object storage or Parquet cold snapshots, storing individual raw request events incurs significant metadata and query scan overhead for high-level historical analysis.

The PySpark job:
1. Ingests raw partition Parquet files adhering to the `edge_http_logs` schema.
2. Truncates timestamps to hourly boundaries (`rollup_hour`).
3. Groups by `rollup_hour`, `tenant_id`, `edge_colo`, `status_code`, and `country`.
4. Computes `total_requests`, `total_egress_bytes`, `avg_duration_ms`, `min_duration_ms`, and `max_duration_ms`.
5. Writes partitioned Parquet files partitioned by `tenant_id` for downstream BI tools or query engines.

---

## 🚀 How to Run Locally

### 1. Prerequisites
* Python 3.10+
* Java 11 or 17+ (required by PySpark for local JVM execution)

### 2. Setup Virtual Environment & Dependencies
```bash
cd batch

# Create and activate virtual environment
python -m venv .venv

# On Linux/macOS:
source .venv/bin/activate
# On Windows (PowerShell):
.venv\Scripts\Activate.ps1

# Install requirements
pip install -r requirements.txt
```

### 3. Generate Sample Cold Partition Data
A generator script is provided to synthesize sample partition Parquet data:
```bash
python generate_sample_parquet.py
```
This generates `batch/data/raw_partitions/sample_partition.parquet` with 2,500 synthetic request records.

### 4. Run the Rollup Job
Execute the Spark job in local standalone mode:
```bash
python rollup_job.py --input ./data/raw_partitions --output ./data/hourly_rollups
```

Optional arguments:
* `--input <path>`: Directory or file containing source Parquet partitions (default: `./data/raw_partitions`).
* `--output <path>`: Destination directory for consolidated rollups (default: `./data/hourly_rollups`).
* `--format <parquet|csv>`: Output format (default: `parquet`).

---

## 📊 Sample Output Schema

| Field | Type | Description |
| :--- | :--- | :--- |
| `rollup_hour` | Timestamp | Hourly window (e.g. `2026-10-23 09:00:00`) |
| `tenant_id` | String | Tenant partition key |
| `edge_colo` | String | Edge datacenter code (e.g. `SJC`, `LHR`, `BLR`) |
| `status_code` | Integer | HTTP response status code |
| `country` | String | ISO country code |
| `total_requests` | Long | Total request volume in the hour |
| `total_egress_bytes` | Long | Sum of egress response payload bytes |
| `avg_duration_ms` | Float | Average edge processing latency |
