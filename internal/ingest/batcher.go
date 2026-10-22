package ingest

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/uday68/flarehouse/internal/clickhouse"
	"github.com/uday68/flarehouse/internal/model"
	"github.com/uday68/flarehouse/internal/telemetry"
)

type BatcherConfig struct {
	BatchSize     int
	FlushInterval time.Duration
	QueueCapacity int
	Workers       int
}

type IngestionEngine struct {
	cfg        BatcherConfig
	chClient   *clickhouse.Client
	eventChan  chan model.TelemetryEvent
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex

	// Atomic Telemetry Metrics
	totalReceived atomic.Uint64
	totalFlushed  atomic.Uint64
	batchesCount  atomic.Uint64
	droppedEvents atomic.Uint64
	totalFlushMs  atomic.Uint64
	lastSecFlushes atomic.Uint64
	currentEPS    atomic.Uint64

	// Load testing control
	isSimulating atomic.Bool
	stopSim      chan struct{}
}

func NewIngestionEngine(cfg BatcherConfig, chClient *clickhouse.Client) *IngestionEngine {
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

	ctx, cancel := context.WithCancel(context.Background())

	engine := &IngestionEngine{
		cfg:       cfg,
		chClient:  chClient,
		eventChan: make(chan model.TelemetryEvent, cfg.QueueCapacity),
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

// Ingest adds an event to the buffer; returns false if channel is full (backpressure)
func (e *IngestionEngine) Ingest(event model.TelemetryEvent) bool {
	e.totalReceived.Add(1)
	telemetry.EventsIngestedTotal.Inc()
	select {
	case e.eventChan <- event:
		telemetry.BufferDepthGauge.Set(float64(len(e.eventChan)))
		return true
	default:
		e.droppedEvents.Add(1)
		return false
	}
}

// workerLoop drains events and flushes batched slices to ClickHouse
func (e *IngestionEngine) workerLoop(workerID int) {
	defer e.wg.Done()

	buffer := make([]model.TelemetryEvent, 0, e.cfg.BatchSize)
	ticker := time.NewTicker(e.cfg.FlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(buffer) == 0 {
			return
		}
		flushStart := time.Now()
		batchCopy := make([]model.TelemetryEvent, len(buffer))
		copy(batchCopy, buffer)
		buffer = buffer[:0] // reset slice header

		if err := e.chClient.InsertBatch(e.ctx, batchCopy); err != nil {
			fmt.Printf("[Worker %d] ClickHouse batch insert failed: %v\n", workerID, err)
		} else {
			duration := time.Since(flushStart).Milliseconds()
			e.totalFlushed.Add(uint64(len(batchCopy)))
			e.batchesCount.Add(1)
			e.totalFlushMs.Add(uint64(duration))
			e.lastSecFlushes.Add(uint64(len(batchCopy)))

			// Prometheus Observability metrics
			telemetry.EventsFlushedTotal.Add(float64(len(batchCopy)))
			telemetry.BatchesWrittenTotal.Inc()
			telemetry.BatchFlushLatency.Observe(time.Since(flushStart).Seconds())
			telemetry.BufferDepthGauge.Set(float64(len(e.eventChan)))
		}
	}

	for {
		select {
		case <-e.ctx.Done():
			flush()
			return
		case event, ok := <-e.eventChan:
			if !ok {
				flush()
				return
			}
			buffer = append(buffer, event)
			if len(buffer) >= e.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
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
				time.Sleep(1 * time.Millisecond) // micro throttle to emulate continuous flow
			}
		}
	}()
}

// StopLoadSimulator halts generation
func (e *IngestionEngine) StopLoadSimulator() {
	e.isSimulating.Store(false)
}

// Stop shuts down the engine cleanly
func (e *IngestionEngine) Stop() {
	e.cancel()
	close(e.eventChan)
	e.wg.Wait()
}
