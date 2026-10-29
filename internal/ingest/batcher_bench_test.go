package ingest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/uday68/flarehouse/internal/model"
)

type noopBatchWriter struct{}

func (n *noopBatchWriter) InsertBatch(ctx context.Context, events []model.TelemetryEvent) error {
	return nil
}

func BenchmarkIngestionEngine_IngestThroughput(b *testing.B) {
	writer := &noopBatchWriter{}
	cfg := BatcherConfig{
		BatchSize:     5000,
		FlushInterval: 100 * time.Millisecond,
		QueueCapacity: 500000,
		Workers:       4,
	}

	engine := NewIngestionEngine(cfg, writer)
	engine.Start()
	defer engine.Stop()

	evt := model.TelemetryEvent{
		TenantID:      "tenant-bench",
		RequestID:     "req-bench-uuid-001",
		ClientIP:      "10.0.0.1",
		Host:          "api.example.com",
		Path:          "/v1/bench",
		Method:        "POST",
		StatusCode:    200,
		ResponseBytes: 1024,
		DurationMs:    12.5,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = engine.Ingest(evt)
		}
	})
}

func BenchmarkDeadLetterSpool_Append(b *testing.B) {
	spool := NewDeadLetterSpool(1000000)
	events := make([]model.TelemetryEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = model.TelemetryEvent{RequestID: fmt.Sprintf("req-%d", i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spool.Append(events)
	}
}
