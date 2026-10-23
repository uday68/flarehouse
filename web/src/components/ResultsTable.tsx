import { Zap } from 'lucide-react';
import type { QueryResponse } from '../types';

interface ResultsTableProps {
  queryResult: QueryResponse | null;
}

export function ResultsTable({ queryResult }: ResultsTableProps) {
  if (!queryResult || queryResult.query_blocked) {
    return null;
  }

  return (
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
          <span>
            Duration: <strong style={{ color: '#fff' }}>{queryResult.duration_ms?.toFixed(2) ?? '0.00'} ms</strong>
          </span>
          <span>
            Rows: <strong style={{ color: '#fff' }}>{queryResult.rows ?? 0}</strong>
          </span>
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
                        {typeof val === 'object' && val !== null ? JSON.stringify(val) : String(val ?? '')}
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
  );
}
