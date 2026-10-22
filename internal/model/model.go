package model

import "time"

// TelemetryEvent represents an HTTP edge telemetry event processed by Cloudflare-style networks
type TelemetryEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	TenantID      string    `json:"tenant_id"`
	RequestID     string    `json:"request_id"`
	ClientIP      string    `json:"client_ip"`
	Host          string    `json:"host"`
	Path          string    `json:"path"`
	Method        string    `json:"method"`
	StatusCode    uint16    `json:"status_code"`
	ResponseBytes uint64    `json:"response_bytes"`
	DurationMs    float32   `json:"duration_ms"`
	EdgeColo      string    `json:"edge_colo"`
	Country       string    `json:"country"`
	CacheStatus   string    `json:"cache_status"`
	ThreatScore   uint8     `json:"threat_score"`
}

// QueryRequest contains an analytical query sent to the gateway
type QueryRequest struct {
	SQL         string `json:"sql"`
	BypassCache bool   `json:"bypass_cache"`
}

// QueryResponse contains data and query execution statistics
type QueryResponse struct {
	Columns        []string        `json:"columns"`
	Data           [][]interface{} `json:"data"`
	Rows           int             `json:"rows"`
	DurationMs     float64         `json:"duration_ms"`
	Cached         bool            `json:"cached"`
	CacheKey       string          `json:"cache_key,omitempty"`
	RowsRead       uint64          `json:"rows_read"`
	BytesRead      uint64          `json:"bytes_read"`
	ExecutionTime  float64         `json:"execution_time_sec"`
	QueryBlocked   bool            `json:"query_blocked,omitempty"`
	GuardrailError string          `json:"guardrail_error,omitempty"`
}

// GuardrailDecision describes whether a query is safe to execute
type GuardrailDecision struct {
	Safe               bool     `json:"safe"`
	ViolationReasons   []string `json:"violation_reasons"`
	EstimatedRowsScan  uint64   `json:"estimated_rows_scan"`
	EstimatedBytesScan uint64   `json:"estimated_bytes_scan"`
	HasPartitionFilter bool     `json:"has_partition_filter"`
	HasLimit           bool     `json:"has_limit"`
	Warnings           []string `json:"warnings"`
}

// AIDoctorAnalysis provides diagnostic insights and optimization tips for queries
type AIDoctorAnalysis struct {
	OriginalSQL      string   `json:"original_sql"`
	ExplainPlan      string   `json:"explain_plan"`
	IssuesIdentified []string `json:"issues_identified"`
	Recommendations  []string `json:"recommendations"`
	OptimizedSQL     string   `json:"optimized_sql"`
	ExpectedImpact   string   `json:"expected_impact"`
	GeneratedByAI    bool     `json:"generated_by_ai"`
}

// IngestStats provides live metrics on the batch ingestion pipeline
type IngestStats struct {
	TotalEventsReceived uint64  `json:"total_events_received"`
	TotalEventsFlushed  uint64  `json:"total_events_flushed"`
	BatchesWritten      uint64  `json:"batches_written"`
	CurrentBufferSize   int     `json:"current_buffer_size"`
	BufferCapacity      int     `json:"buffer_capacity"`
	AvgFlushLatencyMs   float64 `json:"avg_flush_latency_ms"`
	EventsPerSecond     float64 `json:"events_per_second"`
	ActiveWorkers       int     `json:"active_workers"`
	DroppedEvents       uint64  `json:"dropped_events"`
}

// TablePartitionInfo represents ClickHouse table partition metadata
type TablePartitionInfo struct {
	Database      string  `json:"database"`
	Table         string  `json:"table"`
	Partition     string  `json:"partition"`
	Name          string  `json:"name"`
	PartType      string  `json:"part_type"`
	Rows          uint64  `json:"rows"`
	BytesOnDisk   uint64  `json:"bytes_on_disk"`
	DataUncomp    uint64  `json:"data_uncompressed_bytes"`
	CompressRatio float64 `json:"compression_ratio"`
}
