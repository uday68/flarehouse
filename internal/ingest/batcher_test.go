package ingest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uday68/flarehouse/internal/model"
)

type mockBatchWriter struct {
	mu           sync.Mutex
	flushed      []model.TelemetryEvent
	failAttempts int
	calls        int
	shouldFail   bool
	errToReturn  error
}

func (m *mockBatchWriter) InsertBatch(ctx context.Context, events []model.TelemetryEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls++

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if m.shouldFail {
		return errors.New("simulated database down")
	}

	if m.failAttempts > 0 {
		m.failAttempts--
		if m.errToReturn != nil {
			return m.errToReturn
		}
		return errors.New("transient network timeout")
	}

	m.flushed = append(m.flushed, events...)
	return nil
}

func (m *mockBatchWriter) FlushedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.flushed)
}

func (m *mockBatchWriter) TotalCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// 1. Successful batch flush
func TestBatcher_SuccessfulBatchFlush(t *testing.T) {
	writer := &mockBatchWriter{}
	cfg := BatcherConfig{
		BatchSize:     5,
		FlushInterval: 50 * time.Millisecond,
		QueueCapacity: 100,
		Workers:       1,
	}

	engine := NewIngestionEngine(cfg, writer)
	engine.Start()
	defer engine.Stop()

	for i := 0; i < 5; i++ {
		engine.Ingest(model.TelemetryEvent{
			RequestID: fmt.Sprintf("req-%d", i),
			ClientIP:  "192.168.1.1",
		})
	}

	// Allow worker to flush batch
	time.Sleep(100 * time.Millisecond)

	if writer.FlushedCount() != 5 {
		t.Fatalf("expected 5 flushed events, got %d", writer.FlushedCount())
	}
	if writer.TotalCalls() != 1 {
		t.Fatalf("expected 1 insert batch call, got %d", writer.TotalCalls())
	}
}

// 2. Transient database failure followed by success (exponential backoff retry)
func TestBatcher_TransientFailure_ExponentialBackoffRetry(t *testing.T) {
	writer := &mockBatchWriter{
		failAttempts: 2, // Fails first 2 attempts, succeeds on 3rd attempt
	}
	cfg := BatcherConfig{
		BatchSize:      3,
		FlushInterval:  50 * time.Millisecond,
		QueueCapacity:  100,
		Workers:        1,
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
	}

	engine := NewIngestionEngine(cfg, writer)
	engine.Start()
	defer engine.Stop()

	for i := 0; i < 3; i++ {
		engine.Ingest(model.TelemetryEvent{RequestID: fmt.Sprintf("req-retry-%d", i)})
	}

	time.Sleep(200 * time.Millisecond)

	if writer.FlushedCount() != 3 {
		t.Fatalf("expected 3 flushed events after retry, got %d", writer.FlushedCount())
	}
	if writer.TotalCalls() != 3 {
		t.Fatalf("expected 3 total calls (2 failed + 1 success), got %d", writer.TotalCalls())
	}
	if engine.retryAttempts.Load() != 2 {
		t.Fatalf("expected 2 retry attempts tracked, got %d", engine.retryAttempts.Load())
	}
}

// 3. Persistent database failure: retries exhausted and routed to DLQ spool
func TestBatcher_PersistentFailure_DeadLetterSpool(t *testing.T) {
	writer := &mockBatchWriter{
		shouldFail: true,
	}
	cfg := BatcherConfig{
		BatchSize:      4,
		FlushInterval:  50 * time.Millisecond,
		QueueCapacity:  100,
		Workers:        1,
		MaxRetries:     2,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
	}

	engine := NewIngestionEngine(cfg, writer)
	engine.Start()
	defer engine.Stop()

	for i := 0; i < 4; i++ {
		engine.Ingest(model.TelemetryEvent{RequestID: fmt.Sprintf("req-dlq-%d", i)})
	}

	time.Sleep(200 * time.Millisecond)

	if writer.FlushedCount() != 0 {
		t.Fatalf("expected 0 flushed events on persistent failure, got %d", writer.FlushedCount())
	}
	// 1 initial attempt + 2 retries = 3 calls
	if writer.TotalCalls() != 3 {
		t.Fatalf("expected 3 calls, got %d", writer.TotalCalls())
	}
	if engine.DeadLetterCount() != 4 {
		t.Fatalf("expected 4 events saved to dead-letter spool, got %d", engine.DeadLetterCount())
	}
	if engine.flushFailures.Load() != 1 {
		t.Fatalf("expected 1 flush failure tracked, got %d", engine.flushFailures.Load())
	}
}

// 4. Queue saturation and backpressure
func TestBatcher_QueueSaturation_Backpressure(t *testing.T) {
	writer := &mockBatchWriter{}
	cfg := BatcherConfig{
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
		QueueCapacity: 5,
		Workers:       0, // Do not start workers, let queue fill up
	}

	engine := NewIngestionEngine(cfg, writer)

	accepted := 0
	dropped := 0

	for i := 0; i < 15; i++ {
		ok := engine.Ingest(model.TelemetryEvent{RequestID: fmt.Sprintf("req-bp-%d", i)})
		if ok {
			accepted++
		} else {
			dropped++
		}
	}

	if accepted != 5 {
		t.Fatalf("expected exactly 5 events accepted into buffer, got %d", accepted)
	}
	if dropped != 10 {
		t.Fatalf("expected 10 dropped events due to buffer saturation, got %d", dropped)
	}
	if engine.droppedEvents.Load() != 10 {
		t.Fatalf("expected droppedEvents metric to be 10, got %d", engine.droppedEvents.Load())
	}
}

// 5. Shutdown with buffered events: graceful flush of in-flight buffer
func TestBatcher_GracefulShutdown_FlushesBufferedEvents(t *testing.T) {
	writer := &mockBatchWriter{}
	cfg := BatcherConfig{
		BatchSize:       100, // Large batch size so it won't flush on size
		FlushInterval:   10 * time.Second, // Long flush interval so it won't flush on timer
		QueueCapacity:   100,
		Workers:         1,
		ShutdownTimeout: 2 * time.Second,
	}

	engine := NewIngestionEngine(cfg, writer)
	engine.Start()

	// Ingest 7 events (below batch size 100)
	for i := 0; i < 7; i++ {
		engine.Ingest(model.TelemetryEvent{RequestID: fmt.Sprintf("req-shutdown-%d", i)})
	}

	// Immediate shutdown
	engine.Stop()

	// All 7 events must have been flushed upon clean shutdown
	if writer.FlushedCount() != 7 {
		t.Fatalf("expected all 7 buffered events to be flushed upon shutdown, got %d", writer.FlushedCount())
	}
}

// 6. Cancellation during retry
func TestBatcher_CancellationDuringRetry(t *testing.T) {
	writer := &mockBatchWriter{
		shouldFail: true,
	}
	cfg := BatcherConfig{
		BatchSize:      2,
		FlushInterval:  10 * time.Millisecond,
		QueueCapacity:  50,
		Workers:        1,
		MaxRetries:     10,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
	}

	engine := NewIngestionEngine(cfg, writer)
	engine.Start()

	engine.Ingest(model.TelemetryEvent{RequestID: "req-c1"})
	engine.Ingest(model.TelemetryEvent{RequestID: "req-c2"})

	time.Sleep(20 * time.Millisecond)

	// Trigger stop while worker is in retry backoff
	engine.Stop()

	// Engine must terminate cleanly without hanging
	if engine.closed.Load() != true {
		t.Fatal("expected engine to be closed cleanly")
	}
}

// 7. Concurrent ingestion and worker shutdown
func TestBatcher_ConcurrentIngestionAndShutdown(t *testing.T) {
	writer := &mockBatchWriter{}
	cfg := BatcherConfig{
		BatchSize:       20,
		FlushInterval:   20 * time.Millisecond,
		QueueCapacity:   5000,
		Workers:         4,
		ShutdownTimeout: 3 * time.Second,
	}

	engine := NewIngestionEngine(cfg, writer)
	engine.Start()

	const numProducers = 10
	const eventsPerProducer = 100
	var wg sync.WaitGroup
	var totalIngested atomic.Uint64

	for p := 0; p < numProducers; p++ {
		wg.Add(1)
		go func(prodID int) {
			defer wg.Done()
			for i := 0; i < eventsPerProducer; i++ {
				ok := engine.Ingest(model.TelemetryEvent{
					RequestID: fmt.Sprintf("prod-%d-%d", prodID, i),
				})
				if ok {
					totalIngested.Add(1)
				}
				time.Sleep(50 * time.Microsecond)
			}
		}(p)
	}

	// Let producers write for a bit, then shut down
	time.Sleep(30 * time.Millisecond)
	wg.Wait()
	engine.Stop()

	flushed := uint64(writer.FlushedCount())
	dropped := engine.droppedEvents.Load()
	ingested := totalIngested.Load()

	// Conservation of events: All accepted events must be either flushed or accounted for
	if flushed+dropped < ingested {
		t.Fatalf("event loss detected: ingested=%d, flushed=%d, dropped=%d", ingested, flushed, dropped)
	}
}
