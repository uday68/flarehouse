import { ShieldAlert } from 'lucide-react';
import type { QueryResponse } from '../types';

interface GuardrailPanelProps {
  queryResult: QueryResponse | null;
  onAskDoctor?: () => void;
}

export function GuardrailPanel({ queryResult, onAskDoctor }: GuardrailPanelProps) {
  if (!queryResult?.query_blocked) {
    return null;
  }

  return (
    <div
      className="card"
      role="alert"
      data-testid="guardrail-alert"
      style={{ borderColor: 'rgba(239, 68, 68, 0.4)', background: '#1c1319' }}
    >
      <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-start' }}>
        <ShieldAlert size={24} color="#ef4444" style={{ flexShrink: 0, marginTop: '2px' }} />
        <div>
          <h4 style={{ color: '#f87171', fontSize: '0.95rem', marginBottom: '4px' }}>
            Query Blocked by Platform Guardrails
          </h4>
          <p style={{ color: '#cbd5e1', fontSize: '0.85rem' }} data-testid="guardrail-error-message">
            {queryResult.guardrail_error}
          </p>
          {onAskDoctor && (
            <div style={{ marginTop: '10px' }}>
              <button
                type="button"
                className="btn btn-secondary"
                style={{ fontSize: '0.75rem', padding: '4px 10px' }}
                onClick={onAskDoctor}
              >
                Ask AI Doctor to rewrite this safely
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
