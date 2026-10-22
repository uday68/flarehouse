import { useState, useEffect } from 'react';
import {
  Activity,
  Database,
  Terminal,
  ShieldAlert,
  Zap,
  Play,
  CheckCircle,
  AlertTriangle,
  Sparkles,
  Layers,
  ArrowRight,
  RefreshCw,
  HardDrive
} from 'lucide-react';
import './App.css';

const API_BASE = 'http://localhost:8080';

const PRESETS = [
  {
    name: '1-Hour Aggregation (SummingMergeTree)',
    desc: 'Uses pre-aggregated Materialized View for instant real-time telemetry rollups',
    sql: `SELECT minute, tenant_id, edge_colo, sum(request_count) as total_requests, sum(total_bytes) as total_bytes\nFROM flarehouse.edge_metrics_1m\nWHERE minute >= now() - INTERVAL 1 HOUR\nGROUP BY minute, tenant_id, edge_colo\nORDER BY minute DESC\nLIMIT 15;`
  },
  {
    name: 'Status 500 Spike Inspection (Partition-Pruned)',
    desc: 'Filters on active partition date and status code',
    sql: `SELECT timestamp, tenant_id, client_ip, host, status_code, edge_colo, duration_ms\nFROM flarehouse.edge_http_logs\nWHERE timestamp >= now() - INTERVAL 12 HOUR AND status_code >= 500\nORDER BY timestamp DESC\nLIMIT 20;`
  },
  {
    name: 'Columnar Anti-Pattern (Triggers Guardrail Warning)',
    desc: 'Unbounded scan with SELECT * violating platform query safety rules',
    sql: `SELECT * FROM flarehouse.edge_http_logs WHERE client_ip = '127.0.0.1';`
  },
  {
    name: 'Legacy Unoptimized Table (Triggers AI Doctor)',
    desc: 'Queries table with ORDER BY tuple() anti-pattern and daily partitions',
    sql: `SELECT * FROM flarehouse.unoptimized_legacy_logs WHERE url_blob LIKE '%login%' ORDER BY timestamp DESC;`
  }
];

interface QueryResponse {
  columns?: string[];
  data?: any[][];
  rows?: number;
  duration_ms?: number;
  cached?: boolean;
  cache_key?: string;
  query_blocked?: boolean;
  guardrail_error?: string;
  error?: string;
}

interface AIDoctorAnalysis {
  original_sql: string;
  explain_plan: string;
  issues_identified: string[];
  recommendations: string[];
  optimized_sql: string;
  expected_impact: string;
  generated_by_ai: boolean;
}

interface IngestStats {
  total_events_received: number;
  total_events_flushed: number;
  batches_written: number;
  current_buffer_size: number;
  buffer_capacity: number;
  avg_flush_latency_ms: number;
  events_per_second: number;
  active_workers: number;
  dropped_events: number;
}

interface PartitionInfo {
  database: string;
  table: string;
  partition: string;
  name: string;
  part_type: string;
  rows: number;
  bytes_on_disk: number;
  data_uncompressed_bytes: number;
  compression_ratio: number;
}

export function App() {
  const [activeTab, setActiveTab] = useState<'query' | 'doctor' | 'streaming' | 'partitions'>('query');
  const [sql, setSql] = useState(PRESETS[0].sql);
  const [bypassCache, setBypassCache] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [queryResult, setQueryResult] = useState<QueryResponse | null>(null);
  
  // AI Doctor State
  const [doctorLoading, setDoctorLoading] = useState(false);
  const [doctorReport, setDoctorReport] = useState<AIDoctorAnalysis | null>(null);

  // Streaming Stats State
  const [stats, setStats] = useState<IngestStats>({
    total_events_received: 0,
    total_events_flushed: 0,
    batches_written: 0,
    current_buffer_size: 0,
    buffer_capacity: 100000,
    avg_flush_latency_ms: 0,
    events_per_second: 0,
    active_workers: 4,
    dropped_events: 0,
  });
  const [isSimulating, setIsSimulating] = useState(false);

  // Partitions
  const [partitions, setPartitions] = useState<PartitionInfo[]>([]);

  // Health
  const [health, setHealth] = useState<{ clickhouse: string; redis: string }>({
    clickhouse: 'connecting...',
    redis: 'connecting...',
  });

  // Fetch health and stats on interval
  useEffect(() => {
    const fetchHealth = async () => {
      try {
        const res = await fetch(`${API_BASE}/health`);
        if (res.ok) {
          const data = await res.json();
          setHealth({ clickhouse: data.clickhouse, redis: data.redis });
        }
      } catch {
        setHealth({ clickhouse: 'offline', redis: 'offline' });
      }
    };

    const fetchStats = async () => {
      try {
        const res = await fetch(`${API_BASE}/api/stats`);
        if (res.ok) {
          const data = await res.json();
          setStats(data);
        }
      } catch {
        // ignore
      }
    };

    fetchHealth();
    fetchStats();
    const interval = setInterval(fetchStats, 1500);
    return () => clearInterval(interval);
  }, []);

  const handleRunQuery = async (queryToRun?: string) => {
    const targetSQL = queryToRun || sql;
    setIsLoading(true);
    setQueryResult(null);

    try {
      const res = await fetch(`${API_BASE}/api/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sql: targetSQL, bypass_cache: bypassCache })
      });
      const data = await res.json();
      setQueryResult(data);
    } catch (err: any) {
      setQueryResult({ error: err.message || 'Failed to connect to gateway' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleRunAIDoctor = async (queryToRun?: string) => {
    const targetSQL = queryToRun || sql;
    setDoctorLoading(true);
    setDoctorReport(null);

    try {
      const res = await fetch(`${API_BASE}/api/explain`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sql: targetSQL })
      });
      const data = await res.json();
      setDoctorReport(data);
      setActiveTab('doctor');
    } catch (err: any) {
      console.error(err);
    } finally {
      setDoctorLoading(false);
    }
  };

  const handleStartLoad = async (count: number = 25000) => {
    setIsSimulating(true);
    try {
      await fetch(`${API_BASE}/api/load/start?count=${count}`, { method: 'POST' });
    } catch (e) {
      console.error(e);
    }
  };

  const handleStopLoad = async () => {
    setIsSimulating(false);
    try {
      await fetch(`${API_BASE}/api/load/stop`, { method: 'POST' });
    } catch (e) {
      console.error(e);
    }
  };

  const fetchPartitions = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/partitions`);
      if (res.ok) {
        const data = await res.json();
        setPartitions(data || []);
      }
    } catch (e) {
      console.error(e);
    }
  };

  return (
    <div className="app-container">
      {/* Top Navbar */}
      <nav className="navbar">
        <div className="nav-brand">
          <div className="cf-logo-badge">
            <Zap size={18} fill="#090d16" />
            <span>CLOUDFLARE</span>
          </div>
          <div>
            <div className="nav-title">FlareHouse Platform Tooling</div>
            <div className="nav-subtitle">Data Intelligence & Analytics • Systems Engineering Gateway</div>
          </div>
        </div>

        <div className="system-status">
          <div className="status-pill">
            <span className={`status-dot ${health.clickhouse === 'healthy' ? '' : 'status-dot-off'}`} />
            <span>ClickHouse: <strong>{health.clickhouse}</strong></span>
          </div>
          <div className="status-pill">
            <span className={`status-dot ${health.redis === 'healthy' ? '' : 'status-dot-off'}`} />
            <span>Redis Cache: <strong>{health.redis}</strong></span>
          </div>
          <div className="status-pill">
            <Layers size={14} color="#f38020" />
            <span>Workers: <strong>{stats.active_workers} Go Daemons</strong></span>
          </div>
        </div>
      </nav>

      {/* Tabs Bar */}
      <div className="tabs-header">
        <button
          className={`tab-btn ${activeTab === 'query' ? 'active' : ''}`}
          onClick={() => setActiveTab('query')}
        >
          <Terminal size={16} />
          <span>Analytical Query Console</span>
        </button>

        <button
          className={`tab-btn ${activeTab === 'doctor' ? 'active' : ''}`}
          onClick={() => setActiveTab('doctor')}
        >
          <Sparkles size={16} />
          <span>AI Query Doctor</span>
          {doctorReport && <span className="badge badge-orange">Analysis Ready</span>}
        </button>

        <button
          className={`tab-btn ${activeTab === 'streaming' ? 'active' : ''}`}
          onClick={() => setActiveTab('streaming')}
        >
          <Activity size={16} />
          <span>Streaming Ingestion Engine</span>
          {stats.events_per_second > 0 && (
            <span className="badge badge-green">{stats.events_per_second.toLocaleString()} eps</span>
          )}
        </button>

        <button
          className={`tab-btn ${activeTab === 'partitions' ? 'active' : ''}`}
          onClick={() => {
            setActiveTab('partitions');
            fetchPartitions();
          }}
        >
          <Database size={16} />
          <span>ClickHouse Partitions</span>
        </button>
      </div>

      {/* Main Workspace */}
      <main className="main-content">
        {/* TAB 1: QUERY CONSOLE */}
        {activeTab === 'query' && (
          <div>
            <div className="card">
              <div className="card-header">
                <div className="card-title">
                  <Terminal size={18} color="#f38020" />
                  <span>Platform Query Gateway with Safe Guardrails</span>
                </div>
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                  <label style={{ fontSize: '0.8rem', color: '#94a3b8', display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={bypassCache}
                      onChange={(e) => setBypassCache(e.target.checked)}
                    />
                    Bypass Redis Cache
                  </label>
                </div>
              </div>

              {/* Presets */}
              <div style={{ marginBottom: '10px' }}>
                <span style={{ fontSize: '0.75rem', color: '#64748b', textTransform: 'uppercase', fontWeight: 600 }}>Preset Scenarios:</span>
                <div className="preset-pills" style={{ marginTop: '6px' }}>
                  {PRESETS.map((p, idx) => (
                    <button
                      key={idx}
                      className="preset-pill"
                      onClick={() => setSql(p.sql)}
                      title={p.desc}
                    >
                      {p.name}
                    </button>
                  ))}
                </div>
              </div>

              {/* SQL Editor */}
              <textarea
                className="query-editor"
                value={sql}
                onChange={(e) => setSql(e.target.value)}
                placeholder="Enter SELECT statement..."
                rows={5}
              />

              <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '12px', alignItems: 'center' }}>
                <div style={{ fontSize: '0.75rem', color: '#64748b' }}>
                  Guardrails enforce partition limits & block unindexed full table scans.
                </div>
                <div style={{ display: 'flex', gap: '10px' }}>
                  <button
                    className="btn btn-secondary"
                    onClick={() => handleRunAIDoctor()}
                    disabled={doctorLoading}
                  >
                    <Sparkles size={15} color="#faad3f" />
                    <span>{doctorLoading ? 'Analyzing...' : 'AI Doctor Diagnosis'}</span>
                  </button>
                  <button
                    className="btn btn-primary"
                    onClick={() => handleRunQuery()}
                    disabled={isLoading}
                  >
                    <Play size={15} />
                    <span>{isLoading ? 'Executing...' : 'Run Query'}</span>
                  </button>
                </div>
              </div>
            </div>

            {/* Guardrail Violation Alert */}
            {queryResult?.query_blocked && (
              <div className="card" style={{ borderColor: 'rgba(239, 68, 68, 0.4)', background: '#1c1319' }}>
                <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-start' }}>
                  <ShieldAlert size={24} color="#ef4444" style={{ flexShrink: 0, marginTop: '2px' }} />
                  <div>
                    <h4 style={{ color: '#f87171', fontSize: '0.95rem', marginBottom: '4px' }}>
                      Query Blocked by Platform Guardrails
                    </h4>
                    <p style={{ color: '#cbd5e1', fontSize: '0.85rem' }}>
                      {queryResult.guardrail_error}
                    </p>
                    <div style={{ marginTop: '10px' }}>
                      <button
                        className="btn btn-secondary"
                        style={{ fontSize: '0.75rem', padding: '4px 10px' }}
                        onClick={() => handleRunAIDoctor()}
                      >
                        Ask AI Doctor to rewrite this safely
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* Query Results */}
            {queryResult && !queryResult.query_blocked && (
              <div className="card">
                <div className="card-header">
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                    <span style={{ fontWeight: 600 }}>Execution Summary</span>
                    {queryResult.cached ? (
                      <span className="badge badge-green" style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                        <Zap size={12} />
                        Cached via Redis (0.5ms)
                      </span>
                    ) : (
                      <span className="badge badge-orange">ClickHouse Native Query</span>
                    )}
                  </div>
                  <div style={{ fontSize: '0.8rem', color: '#94a3b8', display: 'flex', gap: '14px' }}>
                    <span>Duration: <strong style={{ color: '#fff' }}>{queryResult.duration_ms?.toFixed(2)} ms</strong></span>
                    <span>Rows: <strong style={{ color: '#fff' }}>{queryResult.rows}</strong></span>
                  </div>
                </div>

                {queryResult.error ? (
                  <div style={{ color: '#ef4444', fontSize: '0.85rem', padding: '12px', background: '#1e141a', borderRadius: '6px' }}>
                    Error: {queryResult.error}
                  </div>
                ) : (
                  <div className="table-container">
                    <table>
                      <thead>
                        <tr>
                          {queryResult.columns?.map((col, idx) => (
                            <th key={idx}>{col}</th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {queryResult.data && queryResult.data.length > 0 ? (
                          queryResult.data.map((row, rIdx) => (
                            <tr key={rIdx}>
                              {row.map((val, cIdx) => (
                                <td key={cIdx}>
                                  {typeof val === 'object' ? JSON.stringify(val) : String(val ?? '')}
                                </td>
                              ))}
                            </tr>
                          ))
                        ) : (
                          <tr>
                            <td colSpan={queryResult.columns?.length || 1} style={{ textAlign: 'center', color: '#64748b', padding: '24px' }}>
                              No matching rows returned.
                            </td>
                          </tr>
                        )}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* TAB 2: AI QUERY DOCTOR */}
        {activeTab === 'doctor' && (
          <div>
            <div className="card">
              <div className="card-header">
                <div className="card-title">
                  <Sparkles size={20} color="#faad3f" />
                  <span>AI Query Doctor & ClickHouse Index Optimizer</span>
                </div>
                <div style={{ fontSize: '0.75rem', color: '#94a3b8' }}>
                  Analyzes ClickHouse AST and execution plans to fix slow queries and schema bottlenecks
                </div>
              </div>

              {!doctorReport ? (
                <div style={{ textAlign: 'center', padding: '40px 20px', color: '#94a3b8' }}>
                  <Sparkles size={36} color="#64748b" style={{ margin: '0 auto 12px' }} />
                  <p>Run a query or click "AI Doctor Diagnosis" from the Query Console to trigger automated analysis.</p>
                  <button
                    className="btn btn-primary"
                    style={{ marginTop: '16px' }}
                    onClick={() => handleRunAIDoctor()}
                  >
                    Diagnose Current Query
                  </button>
                </div>
              ) : (
                <div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px', marginBottom: '20px' }}>
                    {/* Issues Identified */}
                    <div style={{ background: '#0d1322', border: '1px solid var(--card-border)', borderRadius: '8px', padding: '16px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#f87171', fontWeight: 600, fontSize: '0.9rem', marginBottom: '12px' }}>
                        <AlertTriangle size={16} />
                        <span>Identified Anti-Patterns & Bottlenecks</span>
                      </div>
                      <ul style={{ paddingLeft: '20px', fontSize: '0.85rem', color: '#cbd5e1', lineHeight: '1.6' }}>
                        {doctorReport.issues_identified.map((issue, idx) => (
                          <li key={idx} style={{ marginBottom: '6px' }}>{issue}</li>
                        ))}
                      </ul>
                    </div>

                    {/* Recommendations */}
                    <div style={{ background: '#0d1322', border: '1px solid var(--card-border)', borderRadius: '8px', padding: '16px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#34d399', fontWeight: 600, fontSize: '0.9rem', marginBottom: '12px' }}>
                        <CheckCircle size={16} />
                        <span>Systems Engineering Recommendations</span>
                      </div>
                      <ul style={{ paddingLeft: '20px', fontSize: '0.85rem', color: '#cbd5e1', lineHeight: '1.6' }}>
                        {doctorReport.recommendations.map((rec, idx) => (
                          <li key={idx} style={{ marginBottom: '6px' }}>{rec}</li>
                        ))}
                      </ul>
                    </div>
                  </div>

                  {/* Impact Summary */}
                  <div style={{ background: '#162032', border: '1px solid #23334f', borderRadius: '8px', padding: '12px 16px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
                    <div>
                      <span style={{ fontSize: '0.75rem', color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Estimated Performance Impact:</span>
                      <div style={{ color: '#faad3f', fontWeight: 700, fontSize: '1rem', marginTop: '2px' }}>
                        {doctorReport.expected_impact}
                      </div>
                    </div>
                    {doctorReport.generated_by_ai && (
                      <span className="badge badge-orange">Powered by Google Gemini 2.0</span>
                    )}
                  </div>

                  {/* Optimized SQL */}
                  <div style={{ marginBottom: '20px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                      <span style={{ fontSize: '0.85rem', fontWeight: 600, color: '#e2e8f0' }}>Suggested Optimized SQL:</span>
                      <button
                        className="btn btn-primary"
                        style={{ fontSize: '0.75rem', padding: '5px 12px' }}
                        onClick={() => {
                          setSql(doctorReport.optimized_sql);
                          setActiveTab('query');
                          handleRunQuery(doctorReport.optimized_sql);
                        }}
                      >
                        <ArrowRight size={14} />
                        Apply & Run Optimized Query
                      </button>
                    </div>
                    <pre style={{ background: '#090e1a', border: '1px solid var(--card-border)', padding: '12px', borderRadius: '6px', color: '#38bdf8', fontSize: '0.85rem', overflowX: 'auto' }}>
                      {doctorReport.optimized_sql}
                    </pre>
                  </div>

                  {/* ClickHouse Raw EXPLAIN output */}
                  <div>
                    <span style={{ fontSize: '0.85rem', fontWeight: 600, color: '#94a3b8', marginBottom: '6px', display: 'block' }}>ClickHouse Query Plan (EXPLAIN indexes = 1):</span>
                    <pre style={{ background: '#090e1a', border: '1px solid #1f293d', padding: '10px', borderRadius: '6px', color: '#94a3b8', fontSize: '0.75rem', maxHeight: '150px', overflowY: 'auto' }}>
                      {doctorReport.explain_plan || 'No EXPLAIN plan output recorded.'}
                    </pre>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* TAB 3: STREAMING INGESTION ENGINE */}
        {activeTab === 'streaming' && (
          <div>
            <div className="metric-grid">
              <div className="metric-card">
                <span className="metric-label">Ingestion Throughput</span>
                <span className="metric-value" style={{ color: '#10b981' }}>
                  {stats.events_per_second.toLocaleString()}
                </span>
                <span className="metric-sub">Events per second</span>
              </div>

              <div className="metric-card">
                <span className="metric-label">Total Ingested Events</span>
                <span className="metric-value">
                  {stats.total_events_flushed.toLocaleString()}
                </span>
                <span className="metric-sub">{stats.batches_written} columnar batches</span>
              </div>

              <div className="metric-card">
                <span className="metric-label">Avg Batch Flush Latency</span>
                <span className="metric-value" style={{ color: '#38bdf8' }}>
                  {stats.avg_flush_latency_ms.toFixed(1)} ms
                </span>
                <span className="metric-sub">ClickHouse native insert duration</span>
              </div>

              <div className="metric-card">
                <span className="metric-label">Ring Buffer Usage</span>
                <span className="metric-value" style={{ color: '#faad3f' }}>
                  {((stats.current_buffer_size / stats.buffer_capacity) * 100).toFixed(1)}%
                </span>
                <span className="metric-sub">{stats.current_buffer_size} / {stats.buffer_capacity} events</span>
              </div>
            </div>

            <div className="card">
              <div className="card-header">
                <div className="card-title">
                  <Activity size={18} color="#f38020" />
                  <span>Adaptive Micro-Batching Control Center</span>
                </div>
                <div>
                  {!isSimulating ? (
                    <button
                      className="btn btn-primary"
                      onClick={() => handleStartLoad(25000)}
                    >
                      <Play size={15} />
                      Inject 25,000 Edge Telemetry Events
                    </button>
                  ) : (
                    <button
                      className="btn btn-danger"
                      onClick={() => handleStopLoad()}
                    >
                      Stop Load Generator
                    </button>
                  )}
                </div>
              </div>

              <p style={{ fontSize: '0.85rem', color: '#cbd5e1', lineHeight: '1.6', marginBottom: '16px' }}>
                At Cloudflare scale, sending naive single-row inserts degrades ClickHouse disk merges.
                This engine implements an adaptive Go lock-free buffering worker pool that flushes
                columnar batches (5,000 records or 250ms threshold) with channel-based backpressure.
              </p>

              {/* Progress bar */}
              <div style={{ background: '#172033', height: '12px', borderRadius: '6px', overflow: 'hidden', border: '1px solid var(--card-border)' }}>
                <div
                  style={{
                    height: '100%',
                    width: `${Math.min(100, (stats.current_buffer_size / stats.buffer_capacity) * 100)}%`,
                    background: 'linear-gradient(90deg, #f38020, #faad3f)',
                    transition: 'width 0.2s ease'
                  }}
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', color: '#64748b', marginTop: '6px' }}>
                <span>Buffer Status: Optimal</span>
                <span>Active Parallel Workers: {stats.active_workers} Go Goroutines</span>
              </div>
            </div>
          </div>
        )}

        {/* TAB 4: CLICKHOUSE PARTITIONS */}
        {activeTab === 'partitions' && (
          <div>
            <div className="card">
              <div className="card-header">
                <div className="card-title">
                  <HardDrive size={18} color="#f38020" />
                  <span>ClickHouse Storage & Partition Pruning Telemetry</span>
                </div>
                <button className="btn btn-secondary" onClick={fetchPartitions}>
                  <RefreshCw size={14} />
                  Refresh
                </button>
              </div>

              <p style={{ fontSize: '0.85rem', color: '#94a3b8', marginBottom: '16px' }}>
                Direct telemetry from <code>system.parts</code> for <code>flarehouse.edge_http_logs</code>. Shows sparse indexing parts, row distribution, and ZSTD/LZ4 compression ratios.
              </p>

              <div className="table-container">
                <table>
                  <thead>
                    <tr>
                      <th>Table</th>
                      <th>Partition</th>
                      <th>Part Name</th>
                      <th>Part Type</th>
                      <th>Rows</th>
                      <th>Compressed Size</th>
                      <th>Uncompressed Size</th>
                      <th>Compression Ratio</th>
                    </tr>
                  </thead>
                  <tbody>
                    {partitions.length > 0 ? (
                      partitions.map((p, idx) => (
                        <tr key={idx}>
                          <td><code>{p.table}</code></td>
                          <td><code>{p.partition}</code></td>
                          <td><code style={{ color: '#38bdf8' }}>{p.name}</code></td>
                          <td>{p.part_type}</td>
                          <td style={{ fontWeight: 600 }}>{p.rows.toLocaleString()}</td>
                          <td>{(p.bytes_on_disk / 1024).toFixed(1)} KB</td>
                          <td>{(p.data_uncompressed_bytes / 1024).toFixed(1)} KB</td>
                          <td>
                            <span className="badge badge-green">
                              {p.compression_ratio > 0 ? `${p.compression_ratio.toFixed(2)}x` : '1.0x'}
                            </span>
                          </td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={8} style={{ textAlign: 'center', color: '#64748b', padding: '24px' }}>
                          No active partitions found. Start the load generator to ingest data and write parts.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}

export default App;
