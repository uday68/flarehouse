import { Sparkles, AlertTriangle, CheckCircle, ArrowRight } from 'lucide-react';
import type { AIDoctorAnalysis } from '../types';

interface AIDoctorPanelProps {
  report: AIDoctorAnalysis | null;
  loading: boolean;
  onDiagnose: () => void;
  onApplyOptimized: (sql: string) => void;
}

export function AIDoctorPanel({
  report,
  loading,
  onDiagnose,
  onApplyOptimized,
}: AIDoctorPanelProps) {
  return (
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

      {!report ? (
        <div style={{ textAlign: 'center', padding: '40px 20px', color: '#94a3b8' }}>
          <Sparkles size={36} color="#64748b" style={{ margin: '0 auto 12px' }} />
          <p>Run a query or click "AI Doctor Diagnosis" from the Query Console to trigger automated analysis.</p>
          <button
            type="button"
            className="btn btn-primary"
            style={{ marginTop: '16px' }}
            onClick={onDiagnose}
            disabled={loading}
          >
            {loading ? 'Diagnosing...' : 'Diagnose Current Query'}
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
                {report.issues_identified.map((issue, idx) => (
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
                {report.recommendations.map((rec, idx) => (
                  <li key={idx} style={{ marginBottom: '6px' }}>{rec}</li>
                ))}
              </ul>
            </div>
          </div>

          {/* Impact Summary */}
          <div style={{ background: '#162032', border: '1px solid #23334f', borderRadius: '8px', padding: '12px 16px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
            <div>
              <span style={{ fontSize: '0.75rem', color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Estimated Performance Impact:
              </span>
              <div style={{ color: '#faad3f', fontWeight: 700, fontSize: '1rem', marginTop: '2px' }}>
                {report.expected_impact}
              </div>
            </div>
            {report.generated_by_ai && (
              <span className="badge badge-orange">Powered by Google Gemini 2.0</span>
            )}
          </div>

          {/* Optimized SQL */}
          <div style={{ marginBottom: '20px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
              <span style={{ fontSize: '0.85rem', fontWeight: 600, color: '#e2e8f0' }}>Suggested Optimized SQL:</span>
              <button
                type="button"
                className="btn btn-primary"
                style={{ fontSize: '0.75rem', padding: '5px 12px' }}
                onClick={() => onApplyOptimized(report.optimized_sql)}
              >
                <ArrowRight size={14} />
                Apply & Run Optimized Query
              </button>
            </div>
            <pre style={{ background: '#090e1a', border: '1px solid var(--card-border)', padding: '12px', borderRadius: '6px', color: '#38bdf8', fontSize: '0.85rem', overflowX: 'auto' }}>
              {report.optimized_sql}
            </pre>
          </div>

          {/* ClickHouse Raw EXPLAIN output */}
          <div>
            <span style={{ fontSize: '0.85rem', fontWeight: 600, color: '#94a3b8', marginBottom: '6px', display: 'block' }}>
              ClickHouse Query Plan (EXPLAIN indexes = 1):
            </span>
            <pre style={{ background: '#090e1a', border: '1px solid #1f293d', padding: '10px', borderRadius: '6px', color: '#94a3b8', fontSize: '0.75rem', maxHeight: '150px', overflowY: 'auto' }}>
              {report.explain_plan || 'No EXPLAIN plan output recorded.'}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}
