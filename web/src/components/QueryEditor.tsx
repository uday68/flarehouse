import { Terminal, Sparkles, Play } from 'lucide-react';
import { PresetList } from './PresetList';

interface QueryEditorProps {
  sql: string;
  onChange: (val: string) => void;
  bypassCache: boolean;
  onBypassCacheChange: (val: boolean) => void;
  onRunQuery: () => void;
  onExplain: () => void;
  isLoading: boolean;
  doctorLoading: boolean;
  onSelectPreset: (presetSql: string) => void;
}

export function QueryEditor({
  sql,
  onChange,
  bypassCache,
  onBypassCacheChange,
  onRunQuery,
  onExplain,
  isLoading,
  doctorLoading,
  onSelectPreset,
}: QueryEditorProps) {
  return (
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
              onChange={(e) => onBypassCacheChange(e.target.checked)}
            />
            Bypass Redis Cache
          </label>
        </div>
      </div>

      <PresetList onSelect={onSelectPreset} />

      <textarea
        className="query-editor"
        value={sql}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Enter SELECT statement..."
        rows={5}
        aria-label="SQL Query Input"
      />

      <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '12px', alignItems: 'center' }}>
        <div style={{ fontSize: '0.75rem', color: '#64748b' }}>
          Guardrails enforce partition limits & block unindexed full table scans.
        </div>
        <div style={{ display: 'flex', gap: '10px' }}>
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onExplain}
            disabled={doctorLoading}
          >
            <Sparkles size={15} color="#faad3f" />
            <span>{doctorLoading ? 'Analyzing...' : 'AI Doctor Diagnosis'}</span>
          </button>
          <button
            type="button"
            className="btn btn-primary"
            onClick={onRunQuery}
            disabled={isLoading}
          >
            <Play size={15} />
            <span>{isLoading ? 'Executing...' : 'Run Query'}</span>
          </button>
        </div>
      </div>
    </div>
  );
}
