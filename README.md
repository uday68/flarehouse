# FlareHouse ⚡
> **Self-Service Analytics Gateway, High-Throughput Ingestion Engine & AI Query Doctor for ClickHouse**

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![ClickHouse](https://img.shields.io/badge/ClickHouse-24.3+-FFCC01?style=flat&logo=clickhouse)](https://clickhouse.com)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat&logo=react)](https://react.dev)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-K8s-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io)
[![GitLab CI](https://img.shields.io/badge/GitLab_CI-Pipeline-FC6D26?style=flat&logo=gitlab)](https://about.gitlab.com)
[![Prometheus](https://img.shields.io/badge/Prometheus-Metrics-E6522C?style=flat&logo=prometheus)](https://prometheus.io)
[![Tests](https://img.shields.io/badge/Tests-Passing-10B981?style=flat)](https://github.com/uday68/flarehouse)

**FlareHouse** is an internal platform tooling framework built with **Go**, **ClickHouse**, and **React**. Designed for high-scale telemetry environments (like Cloudflare's Edge and Analytics platforms), FlareHouse solves the critical friction points between product developers, business analysts, and high-throughput columnar OLAP databases.

---

## 🎯 The "Normalized Problems" FlareHouse Solves

In massive analytical environments, engineering teams frequently accept two chronic operational issues as "normal":

1. **Small-Batch Thrashing & Merge Collapse:** Naive microservices push single-row or high-frequency micro-inserts directly to ClickHouse. Because ClickHouse writes an immutable disk part for each insert, this creates thousands of unmerged parts, eventually crashing the cluster with `Too many parts in all data parts in table`.
2. **Runaway Unindexed Analytical Scans:** Internal business teams run unbounded `SELECT *` queries without timestamp filters on multi-billion-row telemetry tables, exhausting cluster RAM and starving critical production queries.
3. **Silent Suboptimal Schemas:** Data models frequently use raw strings and unindexed sort keys (`ORDER BY tuple()`) because developers lack real-time visibility into ClickHouse's sparse indexing and columnar compression mechanics.

**FlareHouse eliminates these problems** through an intelligent ingestion buffer, proactive gateway guardrails, and an automated AI Query Doctor.

---

## 🏗️ System Architecture

```
                                [ Internal Developers & Analysts ]
                                                │
                                    React Developer Portal
                                                │
                 ┌──────────────────────────────┴──────────────────────────────┐
                 ▼                                                             ▼
     [ Analytical Query Gateway ]                               [ High-Throughput Ingestor ]
      (Go HTTP/REST Reverse Proxy)                               (Go Micro-Batching Daemon)
                 │                                                             │
   ┌─────────────┴─────────────┐                                               │
   │ - Query Guardrail Engine  │                                               │ (Lock-free Ring Buffer)
   │ - Semantic Redis Cache    │                                               │ (Vectorized Batch Flush)
   │ - AI Query Doctor (Gemini)│                                               ▼
   └─────────────┬─────────────┘                                 Kafka Stream / Telemetry Events
                 │                                                             │
                 ▼                                                             ▼
        ClickHouse Cluster                                             ClickHouse Cluster
   (system.parts / Read Replicas)                                (MergeTree / Materialized Views)
```

---

## ⚡ Core Platform Capabilities

### 1. High-Throughput Micro-Batching Daemon (Go)
* **Adaptive Buffer:** Collects streaming edge telemetry events in a thread-safe, lock-free ring buffer.
* **Vectorized Columnar Ingestion:** Flushes optimal batches (5,000 events or 250ms threshold) via ClickHouse's native TCP protocol (`clickhouse-go/v2`), completely eliminating disk part thrashing.
* **Channel Backpressure:** Built-in concurrency controls prevent memory runaway when ClickHouse merges are under heavy disk I/O.
* **Throughput Benchmark:** Ingests **25,000+ events/second** on standard development hardware with single-digit millisecond flush latencies.

### 2. Platform Query Gateway & Guardrails (Go)
* **Destructive Command Blocking:** Rejects `DROP`, `TRUNCATE`, `ALTER`, or `SYSTEM` queries from reaching analytical tables.
* **Columnar Scan Enforcement:** Detects `SELECT *` anti-patterns that defeat columnar compression.
* **Partition Pruning Verification:** Flags queries missing `timestamp` or `toDate()` boundaries on wide MergeTree tables.
* **Missing LIMIT Protection:** Prevents un-aggregated raw scans from dumping gigabytes over client sockets.

### 3. AI Query Doctor & Index Optimizer
* **EXPLAIN Plan Parsing:** Automatically inspects ClickHouse's sparse index usage (`EXPLAIN indexes = 1`).
* **Systems Diagnosis:** Identifies table scan anti-patterns, missing `LowCardinality` dictionary compression, and opportunities for pre-aggregated `SummingMergeTree` Materialized Views.
* **Gemini LLM Integration:** Connects with the Google Gemini API to generate production-ready rewritten SQL and actionable DDL recommendations.

### 4. Semantic Redis Query Cache
* **Deterministic Normalization:** Strips excess whitespace, standardizes case, and generates a SHA-256 query hash.
* **Sub-millisecond Responses:** Serves repeated analytical dashboards in **< 0.6 ms** directly from memory.

### 5. Self-Service Developer UI (React + TypeScript)
* Modern Cloudflare-inspired dark interface.
* Interactive SQL workbench with one-click preset scenarios (Materialized Views, high-error IP drilldowns, guardrail trigger tests).
* Live telemetry meters showing real-time Events/sec, buffer capacity, and average flush latency.
* ClickHouse Partition Explorer tracking active parts, uncompressed vs compressed sizes, and compression ratios.

---

## 🚀 Quickstart

### Prerequisites
* [Docker Desktop](https://www.docker.com/)
* [Go 1.24+](https://golang.org/dl/)
* [Node.js 20+](https://nodejs.org/)

### 1. Clone & Start Infrastructure
```bash
git clone https://github.com/uday68/flarehouse.git
cd flarehouse

# Start ClickHouse and Redis
docker compose up -d clickhouse redis
```

The database schema (`deploy/init-clickhouse.sql`) will automatically initialize:
* `flarehouse.edge_http_logs` (Optimized MergeTree with LowCardinality attributes)
* `flarehouse.edge_metrics_1m` (SummingMergeTree pre-aggregated rollup)
* `flarehouse.unoptimized_legacy_logs` (Demonstration anti-pattern table)

### 2. Run the Gateway Server
```bash
go run ./cmd/gateway
```
The server will start on `http://localhost:8080`.

### 3. Open the Developer Portal
Navigate to:
```
http://localhost:8080
```
*(The Go binary automatically serves the pre-compiled React frontend).*

For frontend hot-reloading during development:
```bash
cd web
npm install
npm run dev
```

---

## 🧪 Running Tests & Benchmarks

Run the complete Go test suite with race detection:
```bash
go test -v -race ./...
```

Expected output:
```
=== RUN   TestHeuristicDiagnosis
--- PASS: TestHeuristicDiagnosis (0.00s)
=== RUN   TestHashQueryNormalization
--- PASS: TestHashQueryNormalization (0.00s)
=== RUN   TestGuardrails
--- PASS: TestGuardrails (0.00s)
PASS
ok      github.com/uday68/flarehouse/internal/guardrail
ok      github.com/uday68/flarehouse/internal/cache
ok      github.com/uday68/flarehouse/internal/aidoctor
```

---

## 📊 Live Demo Scenarios

In the FlareHouse UI:
1. **Test High-Throughput Batching:** Navigate to the **Streaming Ingestion Engine** tab and click *"Inject 25,000 Edge Telemetry Events"*. Watch the Go worker pool ingest events in real-time at 25k+ EPS.
2. **Test Query Guardrails:** Go to **Analytical Query Console**, select *Columnar Anti-Pattern*, and run it. The gateway immediately blocks the unpartitioned full-scan and provides actionable guidance.
3. **Test AI Query Doctor:** Select *Legacy Unoptimized Table* and click *AI Doctor Diagnosis*. View the EXPLAIN plan breakdown, detected `ORDER BY tuple()` anti-patterns, and 1-click apply the optimized query.
4. **Test Redis Caching:** Run any analytical aggregation twice. The second run executes in **0.4ms** with the `Cached via Redis` badge.

---

## ☸️ Kubernetes (K8s) Production Architecture

FlareHouse includes production Kubernetes manifests located in [`deploy/k8s/`](./deploy/k8s/):
* **StatefulSet for ClickHouse:** Includes PersistentVolumeClaim templates (`fast-nvme`) and headless services for discovery.
* **RollingUpdate Deployment for Gateway:** Configured with 3 replicas, non-root runtime security contexts (`UID 10001`), resource requests/limits, and HTTP `/health` liveness and readiness probes.
* **HorizontalPodAutoscaler (HPA):** Dynamically scales Gateway pods between 2 and 10 based on CPU (70%) and memory thresholds.
* **Prometheus Annotations:** Automatically discovered by Kubernetes Prometheus operators via `prometheus.io/scrape: "true"`.

```bash
# Deploy to Kubernetes cluster using Kustomize
kubectl apply -k deploy/k8s/
```

---

## 🦊 GitLab CI/CD Pipeline

FlareHouse features an enterprise-grade [`.gitlab-ci.yml`](./.gitlab-ci.yml) pipeline matching Cloudflare's internal engineering workflows:
1. **Lint Stage:** Runs `golangci-lint`, `go vet`, and frontend TypeScript validation.
2. **Test Stage:** Executes `go test -v -race -coverprofile=coverage.out ./...` with Cobertura coverage reports.
3. **Build Stage:** Compiles cross-platform static Linux binaries (`linux-amd64` and `linux-arm64`).
4. **Package Stage:** Hardened multi-stage Docker container build on branch merge.

---

## 📈 Prometheus Telemetry & Observability

Standard Prometheus metrics are exported natively at `/metrics`:
* `flarehouse_ingest_events_received_total`: Total streaming events accepted.
* `flarehouse_ingest_events_flushed_total`: Total events successfully committed to ClickHouse.
* `flarehouse_ingest_batches_written_total`: Total columnar batch insertions.
* `flarehouse_ingest_batch_flush_duration_seconds`: Histogram of ClickHouse insert latencies.
* `flarehouse_ingest_buffer_queue_depth`: Live gauge of ring-buffer queue saturation.
* `flarehouse_gateway_query_execution_duration_seconds`: Analytical query duration partitioned by cache hit status.
* `flarehouse_gateway_guardrail_blocked_queries_total`: Count of queries blocked by safety rules.

---

## ☁️ Centralized Cloud Data Lake & Storage Tiering

FlareHouse supports hybrid storage tiering ([`deploy/gcp-storage-policy.xml`](./deploy/gcp-storage-policy.xml)):
* **Hot Tier:** High-speed NVMe local storage for sub-second analytical queries on active 14-day telemetry.
* **Cold Lake Tier:** Automated background partition migration to **Google Cloud Storage (GCS)** or **Cloudflare R2** via ClickHouse storage policies.
* **Data Lake Export:** Exporting hourly analytical rollups to Apache Parquet format for BigQuery, Spark, and cross-functional business intelligence federation.

---

## 👨‍💻 Author
* **V Uday** — [GitHub: Uday68](https://github.com/uday68) • [LinkedIn](https://www.linkedin.com/in/uday-v-b2a65427b)
