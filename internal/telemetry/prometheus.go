package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// EventsIngestedTotal tracks incoming streaming telemetry events
	EventsIngestedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "events_received_total",
		Help:      "Total count of telemetry events received by the platform buffer",
	})

	// EventsFlushedTotal tracks events successfully written to ClickHouse
	EventsFlushedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "events_flushed_total",
		Help:      "Total count of events flushed to ClickHouse MergeTree storage",
	})

	// BatchesWrittenTotal tracks total columnar batches sent to ClickHouse
	BatchesWrittenTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "batches_written_total",
		Help:      "Total number of columnar batches written to ClickHouse",
	})

	// BatchFlushLatency tracks the time taken for ClickHouse native batch writes
	BatchFlushLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "batch_flush_duration_seconds",
		Help:      "Latency distribution of columnar batch flushes to ClickHouse",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
	})

	// BufferDepthGauge tracks current ring-buffer utilization
	BufferDepthGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "buffer_queue_depth",
		Help:      "Current number of un-flushed events in the Go ring buffer",
	})

	// ActiveWorkersGauge tracks active parallel worker goroutines
	ActiveWorkersGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "active_workers_count",
		Help:      "Number of active parallel Go ingestion worker goroutines",
	})

	// QueryDuration tracks analytical query latencies across cache states
	QueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "flarehouse",
		Subsystem: "gateway",
		Name:      "query_execution_duration_seconds",
		Help:      "Execution latency of analytical queries routed through the gateway",
		Buckets:   []float64{0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 3.0},
	}, []string{"cached", "status"})

	// GuardrailBlockedTotal tracks dangerous or runaway queries blocked by platform rules
	GuardrailBlockedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "flarehouse",
		Subsystem: "gateway",
		Name:      "guardrail_blocked_queries_total",
		Help:      "Total count of queries blocked by platform safety guardrails",
	})

	// BatchFlushFailuresTotal tracks batches that failed after exhausting retries
	BatchFlushFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "batch_flush_failures_total",
		Help:      "Total count of batch flush operations that failed all retries",
	})

	// BatchRetryCountTotal tracks individual retry attempts during flush failures
	BatchRetryCountTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "batch_retry_count_total",
		Help:      "Total number of retry attempts made for failed ClickHouse flushes",
	})

	// EventsDroppedTotal tracks events dropped due to buffer saturation or exhausted retries
	EventsDroppedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "events_dropped_total",
		Help:      "Total count of telemetry events dropped permanently",
	})

	// DLQEventsTotal tracks events diverted to dead-letter quarantine spool
	DLQEventsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "flarehouse",
		Subsystem: "ingest",
		Name:      "dlq_events_total",
		Help:      "Total count of events saved to dead-letter quarantine after persistent failure",
	})
)
