import { Activity, Play } from 'lucide-react';
import type { IngestStatsData } from '../types';

interface IngestStatsProps {
  stats: IngestStatsData;
  isSimulating: boolean;
  onStartLoad: (count?: number) => void;
  onStopLoad: () => void;
}

export function IngestStats({
  stats,
  isSimulating,
  onStartLoad,
  onStopLoad,
}: IngestStatsProps) {
  const bufferPercent = stats.buffer_capacity > 0
    ? Math.min(100, (stats.current_buffer_size / stats.buffer_capacity) * 100)
    : 0;

  return (
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
            {bufferPercent.toFixed(1)}%
          </span>
          <span className="metric-sub">
            {stats.current_buffer_size} / {stats.buffer_capacity} events
          </span>
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
                type="button"
                className="btn btn-primary"
                onClick={() => onStartLoad(25000)}
              >
                <Play size={15} />
                Inject 25,000 Edge Telemetry Events
              </button>
            ) : (
              <button
                type="button"
                className="btn btn-danger"
                onClick={onStopLoad}
              >
                Stop Load Generator
              </button>
            )}
          </div>
        </div>

        <p style={{ fontSize: '0.85rem', color: '#cbd5e1', lineHeight: '1.6', marginBottom: '16px' }}>
          In high-scale OLAP environments, individual row inserts cause merge tree part fragmentation.
          This engine uses an adaptive Go lock-free buffering worker pool that flushes
          columnar batches (5,000 records or 250ms threshold) with channel-based backpressure.
        </p>

        {/* Progress bar */}
        <div style={{ background: '#172033', height: '12px', borderRadius: '6px', overflow: 'hidden', border: '1px solid var(--card-border)' }}>
          <div
            style={{
              height: '100%',
              width: `${bufferPercent}%`,
              background: 'linear-gradient(90deg, #f38020, #faad3f)',
              transition: 'width 0.2s ease',
            }}
          />
        </div>

        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', color: '#64748b', marginTop: '6px' }}>
          <span>Buffer Status: Optimal</span>
          <span>Active Parallel Workers: {stats.active_workers} Go Goroutines</span>
        </div>
      </div>
    </div>
  );
}
