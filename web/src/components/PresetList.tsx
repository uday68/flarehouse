import type { Preset } from '../types';

export const PRESETS: Preset[] = [
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

interface PresetListProps {
  onSelect: (sql: string) => void;
}

export function PresetList({ onSelect }: PresetListProps) {
  return (
    <div style={{ marginBottom: '10px' }}>
      <span style={{ fontSize: '0.75rem', color: '#64748b', textTransform: 'uppercase', fontWeight: 600 }}>
        Preset Scenarios:
      </span>
      <div className="preset-pills" style={{ marginTop: '6px' }}>
        {PRESETS.map((p, idx) => (
          <button
            key={idx}
            className="preset-pill"
            onClick={() => onSelect(p.sql)}
            title={p.desc}
          >
            {p.name}
          </button>
        ))}
      </div>
    </div>
  );
}
