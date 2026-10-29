package ingest

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/uday68/flarehouse/internal/model"
	"github.com/uday68/flarehouse/internal/telemetry"
)

// BatchWriter defines the minimal interface for persistent telemetry batch storage
type BatchWriter interface {
	InsertBatch(ctx context.Context, events []model.TelemetryEvent) error
}

type BatcherConfig struct {
	BatchSize       int
	FlushInterval   time.Duration
	QueueCapacity   int
	Workers         int
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	ShutdownTimeout time.Duration
}

// DeadLetterSpool stores events that permanently failed persistent insertion
type DeadLetterSpool struct {
	mu     sync.RWMutex
	events []model.TelemetryEvent
	maxCap int
}

func NewDeadLetterSpool(capacity int) *DeadLetterSpool {
	if capacity <= 0 {
		capacity = 50000
	}
	return &DeadLetterSpool{
		events: make([]model.TelemetryEvent, 0, 1024),
		maxCap: capacity,
	}
}

func (s *DeadLetterSpool) Append(events []model.TelemetryEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	available := s.maxCap - len(s.events)
	if available <= 0 {
		return
	}
	if len(events) > available {
		s.events = append(s.events, events[:available]...)
	} else {
		s.events = append(s.events, events...)
	}
}

func (s *DeadLetterSpool) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

func (s *DeadLetterSpool) Drain() []model.TelemetryEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	drained := s.events
	s.events = make([]model.TelemetryEvent, 0, 1024)
	return drained
}

type IngestionEngine struct {
	cfg        BatcherConfig
	writer     BatchWriter
	eventChan  chan model.TelemetryEvent
	dlqSpool   *DeadLetterSpool
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	closed     atomic.Bool

	// Atomic Telemetry Metrics
	totalReceived  atomic.Uint64
	totalFlushed   atomic.Uint64
	batchesCount   atomic.Uint64
	droppedEvents  atomic.Uint64
	totalFlushMs   atomic.Uint64
	lastSecFlushes atomic.Uint64
	currentEPS     atomic.Uint64
	retryAttempts  atomic.Uint64
	flushFailures  atomic.Uint64

	// Load testing control
	isSimulating atomic.Bool
	stopSim      chan struct{}
}

func NewIngestionEngine(cfg BatcherConfig, writer BatchWriter) *IngestionEngine {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 5000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 300 * time.Millisecond
	}
	if cfg.QueueCapacity <= 0 {
		cfg.QueueCapacity = 50000
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 25 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 500 * time.Millisecond
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 5 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &IngestionEngine{
		cfg:       cfg,
		writer:    writer,
		eventChan: make(chan model.TelemetryEvent, cfg.QueueCapacity),
		dlqSpool:  NewDeadLetterSpool(cfg.QueueCapacity),
		ctx:       ctx,
		cancel:    cancel,
		stopSim:   make(chan struct{}),
	}

	return engine
}

// Start spawns background ingestion workers
func (e *IngestionEngine) Start() {
	telemetry.ActiveWorkersGauge.Set(float64(e.cfg.Workers))
	for i := 0; i < e.cfg.Workers; i++ {
		e.wg.Add(1)
		go e.workerLoop(i)
	}

	// Rate calculation ticker
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-e.ctx.Done():
				return
			case <-ticker.C:
				diff := e.lastSecFlushes.Swap(0)
				e.currentEPS.Store(diff)
				telemetry.BufferDepthGauge.Set(float64(len(e.eventChan)))
			}
		}
	}()
}

// Ingest adds an event to the buffer; returns false if channel is full or engine is closing (backpressure)
func (e *IngestionEngine) Ingest(event model.TelemetryEvent) bool {
	if e.closed.Load() {
		e.droppedEvents.Add(1)
		telemetry.EventsDroppedTotal.Inc()
		return false
	}

	e.totalReceived.Add(1)
	telemetry.EventsIngestedTotal.Inc()

	select {
	case e.eventChan <- event:
		telemetry.BufferDepthGauge.Set(float64(len(e.eventChan)))
		return true
	default:
		e.droppedEvents.Add(1)
		telemetry.EventsDroppedTotal.Inc()
		return false
	}
}

// flushBatchWithRetry attempts bounded retries with exponential backoff and jitter
func (e *IngestionEngine) flushBatchWithRetry(ctx context.Context, batch []model.TelemetryEvent, workerID int) error {
	if len(batch) == 0 {
		return nil
	}

	flushStart := time.Now()
	var lastErr error

	for attempt := 0; attempt <= e.cfg.MaxRetries; attempt++ {
		// Stop retrying if the context is cancelled
		if ctx.Err() != nil {
			e.droppedEvents.Add(uint64(len(batch)))
			telemetry.EventsDroppedTotal.Add(float64(len(batch)))
			return ctx.Err()
		}

		if attempt > 0 {
			e.retryAttempts.Add(1)
			telemetry.BatchRetryCountTotal.Inc()

			// Exponential backoff with full jitter
			backoff := e.cfg.InitialBackoff * (1 << (attempt - 1))
			if backoff > e.cfg.MaxBackoff {
				backoff = e.cfg.MaxBackoff
			}
			jitter := time.Duration(rand.Int63n(int64(backoff) + 1))
			sleepDur := (backoff / 2) + (jitter / 2)

			select {
			case <-ctx.Done():
				e.droppedEvents.Add(uint64(len(batch)))
				telemetry.EventsDroppedTotal.Add(float64(len(batch)))
				return ctx.Err()
			case <-time.After(sleepDur):
			}
		}

		if e.writer == nil {
			lastErr = fmt.Errorf("no batch writer configured")
			continue
		}

		err := e.writer.InsertBatch(ctx, batch)
		if err == nil {
			duration := time.Since(flushStart).Milliseconds()
			e.totalFlushed.Add(uint64(len(batch)))
			e.batchesCount.Add(1)
			e.totalFlushMs.Add(uint64(duration))
			e.lastSecFlushes.Add(uint64(len(batch)))

			telemetry.EventsFlushedTotal.Add(float64(len(batch)))
			telemetry.BatchesWrittenTotal.Inc()
			telemetry.BatchFlushLatency.Observe(time.Since(flushStart).Seconds())
			telemetry.BufferDepthGauge.Set(float64(len(e.eventChan)))
			return nil
		}

		lastErr = err
	}

	// Max retries exhausted: route batch to dead-letter spool
	e.flushFailures.Add(1)
	e.droppedEvents.Add(uint64(len(batch)))
	telemetry.BatchFlushFailuresTotal.Inc()
	telemetry.DLQEventsTotal.Add(float64(len(batch)))
	e.dlqSpool.Append(batch)

	return fmt.Errorf("worker %d: batch flush failed after %d retries: %w", workerID, e.cfg.MaxRetries, lastErr)
}

// workerLoop drains events and flushes batched slices
func (e *IngestionEngine) workerLoop(workerID int) {
	defer e.wg.Done()

	buffer := make([]model.TelemetryEvent, 0, e.cfg.BatchSize)
	ticker := time.NewTicker(e.cfg.FlushInterval)
	defer ticker.Stop()

	flush := func(ctx context.Context) {
		if len(buffer) == 0 {
			return
		}
		batchCopy := make([]model.TelemetryEvent, len(buffer))
		copy(batchCopy, buffer)
		buffer = buffer[:0] // reset slice header only after copying

		_ = e.flushBatchWithRetry(ctx, batchCopy, workerID)
	}

	for {
		select {
		case event, ok := <-e.eventChan:
			if !ok {
				// Channel closed, flush remaining buffer using bounded shutdown context
				shutdownCtx, cancel := context.WithTimeout(context.Background(), e.cfg.ShutdownTimeout)
				flush(shutdownCtx)
				cancel()
				return
			}
			buffer = append(buffer, event)
			if len(buffer) >= e.cfg.BatchSize {
				flush(e.ctx)
			}
		case <-ticker.C:
			flush(e.ctx)
		}
	}
}

// DeadLetterCount returns the number of events in the dead-letter spool
func (e *IngestionEngine) DeadLetterCount() int {
	return e.dlqSpool.Len()
}

// DrainDeadLetterEvents retrieves and clears dead-letter events for inspection or replay
func (e *IngestionEngine) DrainDeadLetterEvents() []model.TelemetryEvent {
	return e.dlqSpool.Drain()
}

// Stats returns current operational metrics
func (e *IngestionEngine) Stats() model.IngestStats {
	batches := e.batchesCount.Load()
	var avgLatency float64
	if batches > 0 {
		avgLatency = float64(e.totalFlushMs.Load()) / float64(batches)
	}

	return model.IngestStats{
		TotalEventsReceived: e.totalReceived.Load(),
		TotalEventsFlushed:  e.totalFlushed.Load(),
		BatchesWritten:      batches,
		CurrentBufferSize:   len(e.eventChan),
		BufferCapacity:      e.cfg.QueueCapacity,
		AvgFlushLatencyMs:   avgLatency,
		EventsPerSecond:     float64(e.currentEPS.Load()),
		ActiveWorkers:       e.cfg.Workers,
		DroppedEvents:       e.droppedEvents.Load(),
	}
}

// StartLoadSimulator generates synthetic edge telemetry in the background
func (e *IngestionEngine) StartLoadSimulator(count int) {
	if !e.isSimulating.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer e.isSimulating.Store(false)
		colos := []string{"SJC", "IAD", "LHR", "FRA", "SIN", "BLR", "NRT", "SYD"}
		methods := []string{"GET", "POST", "GET", "GET", "HEAD", "PUT"}
		statuses := []uint16{200, 200, 200, 204, 301, 403, 404, 500}
		cacheStates := []string{"HIT", "MISS", "EXPIRED", "BYPASS", "REVALIDATED"}
		tenants := []string{"cloudflare-internal", "enterprise-corp", "fast-startup-io", "global-bank"}
		countries := []string{"IN", "US", "GB", "DE", "SG", "JP", "AU"}

		for i := 0; i < count; i++ {
			if !e.isSimulating.Load() {
				break
			}

			evt := model.TelemetryEvent{
				Timestamp:     time.Now().UTC(),
				TenantID:      tenants[rand.Intn(len(tenants))],
				RequestID:     uuid.New().String(),
				ClientIP:      fmt.Sprintf("%d.%d.%d.%d", rand.Intn(220)+1, rand.Intn(255), rand.Intn(255), rand.Intn(255)),
				Host:          fmt.Sprintf("api.service-%d.cloudflare.net", rand.Intn(10)),
				Path:          fmt.Sprintf("/v1/analytics/stream/%d", rand.Intn(500)),
				Method:        methods[rand.Intn(len(methods))],
				StatusCode:    statuses[rand.Intn(len(statuses))],
				ResponseBytes: uint64(rand.Intn(100000) + 200),
				DurationMs:    float32(rand.Float64()*120.0 + 2.0),
				EdgeColo:      colos[rand.Intn(len(colos))],
				Country:       countries[rand.Intn(len(countries))],
				CacheStatus:   cacheStates[rand.Intn(len(cacheStates))],
				ThreatScore:   uint8(rand.Intn(10)),
			}

			e.Ingest(evt)
			if i%1000 == 0 {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()
}

// StopLoadSimulator halts generation
func (e *IngestionEngine) StopLoadSimulator() {
	e.isSimulating.Store(false)
}

// Stop cleanly drains remaining events, flushes buffers, and halts workers
func (e *IngestionEngine) Stop() {
	if !e.closed.CompareAndSwap(false, true) {
		return
	}

	// Close channel to signal workers to drain remaining events
	close(e.eventChan)

	// Wait for workers to flush their remaining buffers with bounded shutdown context
	e.wg.Wait()

	// Cancel background rate ticker context
	e.cancel()
}
