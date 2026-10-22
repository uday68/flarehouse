package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/uday68/flarehouse/internal/aidoctor"
	"github.com/uday68/flarehouse/internal/cache"
	"github.com/uday68/flarehouse/internal/clickhouse"
	"github.com/uday68/flarehouse/internal/guardrail"
	"github.com/uday68/flarehouse/internal/ingest"
	"github.com/uday68/flarehouse/internal/model"
	"github.com/uday68/flarehouse/internal/telemetry"
)

type Server struct {
	chClient   *clickhouse.Client
	queryCache *cache.QueryCache
	guardrails *guardrail.GuardrailEngine
	aiDoctor   *aidoctor.Doctor
	ingestion  *ingest.IngestionEngine
}

func main() {
	port := getEnv("PORT", "8080")
	chAddr := getEnv("CLICKHOUSE_ADDR", "localhost:9000")
	chDB := getEnv("CLICKHOUSE_DB", "flarehouse")
	chUser := getEnv("CLICKHOUSE_USER", "default")
	chPass := getEnv("CLICKHOUSE_PASSWORD", "clickhouse123")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")

	log.Printf("[FlareHouse Gateway] Starting on port :%s...", port)
	log.Printf("[FlareHouse Gateway] Connecting to ClickHouse at %s (DB: %s)...", chAddr, chDB)

	chClient, err := clickhouse.NewClient(chAddr, chDB, chUser, chPass)
	if err != nil {
		log.Printf("[WARNING] ClickHouse initial connection warning: %v", err)
	} else {
		log.Println("[FlareHouse Gateway] Connected to ClickHouse native protocol.")
	}

	queryCache := cache.NewQueryCache(redisAddr)
	guardrails := guardrail.NewGuardrailEngine()
	aiDoctor := aidoctor.NewDoctor()

	// Ingestion engine with adaptive batching: 5,000 batch size, 250ms flush
	ingestionEngine := ingest.NewIngestionEngine(ingest.BatcherConfig{
		BatchSize:     5000,
		FlushInterval: 250 * time.Millisecond,
		QueueCapacity: 100000,
		Workers:       4,
	}, chClient)
	ingestionEngine.Start()
	defer ingestionEngine.Stop()

	srv := &Server{
		chClient:   chClient,
		queryCache: queryCache,
		guardrails: guardrails,
		aiDoctor:   aiDoctor,
		ingestion:  ingestionEngine,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", srv.handleHealth)
	r.Handle("/metrics", promhttp.Handler())
	r.Post("/api/query", srv.handleQuery)
	r.Post("/api/explain", srv.handleExplain)
	r.Get("/api/stats", srv.handleStats)
	r.Get("/api/partitions", srv.handlePartitions)
	r.Post("/api/load/start", srv.handleStartLoad)
	r.Post("/api/load/stop", srv.handleStopLoad)
	r.Post("/api/ingest", srv.handleIngestEvent)

	// Static Web UI serving (auto-fallback across execution directories)
	staticDirs := []string{"./web/dist", "../web/dist", "../../web/dist"}
	for _, dir := range staticDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			fileServer := http.FileServer(http.Dir(dir))
			r.Handle("/*", fileServer)
			log.Printf("[FlareHouse Gateway] Serving Web UI from %s", dir)
			break
		}
	}

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[FlareHouse Gateway] HTTP listening on http://localhost:%s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server listen error: %v", err)
		}
	}()

	<-stop
	log.Println("[FlareHouse Gateway] Gracefully shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	if chClient != nil {
		chClient.Close()
	}
	log.Println("[FlareHouse Gateway] Server stopped.")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	chStatus := "healthy"
	if s.chClient == nil || s.chClient.Ping(ctx) != nil {
		chStatus = "unreachable"
	}

	redisStatus := "healthy"
	if s.queryCache == nil || s.queryCache.Ping(ctx) != nil {
		redisStatus = "unreachable"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "healthy",
		"service":    "flarehouse-gateway",
		"clickhouse": chStatus,
		"redis":      redisStatus,
		"timestamp":  time.Now().UTC(),
	})
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req model.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SQL == "" {
		http.Error(w, "sql query is required", http.StatusBadRequest)
		return
	}

	// 1. Guardrail evaluation
	decision := s.guardrails.Evaluate(req.SQL)
	if !decision.Safe {
		telemetry.GuardrailBlockedTotal.Inc()
		writeJSON(w, http.StatusOK, model.QueryResponse{
			QueryBlocked:   true,
			GuardrailError: fmt.Sprintf("Guardrail Violation: %s", decision.ViolationReasons[0]),
		})
		return
	}

	cacheKey := cache.HashQuery(req.SQL)

	// 2. Cache lookup
	if !req.BypassCache {
		if cachedResp, err := s.queryCache.Get(r.Context(), cacheKey); err == nil && cachedResp != nil {
			telemetry.QueryDuration.WithLabelValues("true", "200").Observe(cachedResp.DurationMs / 1000.0)
			writeJSON(w, http.StatusOK, cachedResp)
			return
		}
	}

	// 3. Execution against ClickHouse
	resp, err := s.chClient.ExecuteQuery(r.Context(), req.SQL)
	if err != nil {
		telemetry.QueryDuration.WithLabelValues("false", "500").Observe(0)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":   err.Error(),
			"message": "ClickHouse execution failed",
		})
		return
	}

	telemetry.QueryDuration.WithLabelValues("false", "200").Observe(resp.DurationMs / 1000.0)

	// 4. Save to Redis cache for 60 seconds
	_ = s.queryCache.Set(r.Context(), cacheKey, resp, 60*time.Second)

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	var req model.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	explainText, err := s.chClient.ExplainQuery(r.Context(), req.SQL)
	if err != nil {
		explainText = fmt.Sprintf("Explain failed: %v", err)
	}

	analysis := s.aiDoctor.Diagnose(r.Context(), req.SQL, explainText)
	writeJSON(w, http.StatusOK, analysis)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.ingestion.Stats()
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handlePartitions(w http.ResponseWriter, r *http.Request) {
	parts, err := s.chClient.GetTablePartitions(r.Context(), "edge_http_logs")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, parts)
}

func (s *Server) handleStartLoad(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 25000
	if c, err := strconv.Atoi(countStr); err == nil && c > 0 {
		count = c
	}
	s.ingestion.StartLoadSimulator(count)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "load_generator_started",
		"events":  count,
		"message": fmt.Sprintf("Injecting %d synthetic events through adaptive Go ring-buffer...", count),
	})
}

func (s *Server) handleStopLoad(w http.ResponseWriter, r *http.Request) {
	s.ingestion.StopLoadSimulator()
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "load_generator_stopped",
	})
}

func (s *Server) handleIngestEvent(w http.ResponseWriter, r *http.Request) {
	var evt model.TelemetryEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		http.Error(w, "invalid event payload", http.StatusBadRequest)
		return
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}

	accepted := s.ingestion.Ingest(evt)
	if !accepted {
		http.Error(w, "Buffer full - backpressure applied", http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func writeJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
