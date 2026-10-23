import { useState } from 'react';
import {
  Activity,
  Database,
  Terminal,
  Zap,
  Sparkles,
  Layers,
} from 'lucide-react';
import { PRESETS } from './components/PresetList';
import { QueryEditor } from './components/QueryEditor';
import { GuardrailPanel } from './components/GuardrailPanel';
import { ResultsTable } from './components/ResultsTable';
import { AIDoctorPanel } from './components/AIDoctorPanel';
import { IngestStats } from './components/IngestStats';
import { PartitionExplorer } from './components/PartitionExplorer';
import {
  useGetHealthQuery,
  useGetStatsQuery,
  useGetPartitionsQuery,
  useRunQueryMutation,
  useExplainQueryMutation,
  useStartLoadMutation,
  useStopLoadMutation,
} from './store/api';
import type { QueryResponse, AIDoctorAnalysis } from './types';
import './App.css';

export function App() {
  const [activeTab, setActiveTab] = useState<'query' | 'doctor' | 'streaming' | 'partitions'>('query');
  const [sql, setSql] = useState<string>(PRESETS[0].sql);
  const [bypassCache, setBypassCache] = useState<boolean>(false);
  const [queryResult, setQueryResult] = useState<QueryResponse | null>(null);
  const [doctorReport, setDoctorReport] = useState<AIDoctorAnalysis | null>(null);
  const [isSimulating, setIsSimulating] = useState<boolean>(false);

  // RTK Query hooks
  const { data: health } = useGetHealthQuery(undefined, { pollingInterval: 5000 });
  const { data: stats } = useGetStatsQuery(undefined, { pollingInterval: 1500 });
  const { data: partitions = [], isFetching: isPartitionsLoading, refetch: refetchPartitions } = useGetPartitionsQuery();

  const [runQueryMutation, { isLoading: isQueryLoading }] = useRunQueryMutation();
  const [explainMutation, { isLoading: isDoctorLoading }] = useExplainQueryMutation();
  const [startLoadMutation] = useStartLoadMutation();
  const [stopLoadMutation] = useStopLoadMutation();

  const handleRunQuery = async (targetSql?: string) => {
    const queryToExecute = targetSql || sql;
    try {
      const res = await runQueryMutation({ sql: queryToExecute, bypass_cache: bypassCache }).unwrap();
      setQueryResult(res);
    } catch (err: unknown) {
      const errorMessage =
        err && typeof err === 'object' && 'data' in err && typeof (err as { data: { error?: string } }).data === 'object'
          ? (err as { data: { error?: string } }).data.error || 'Execution failed'
          : 'Failed to connect to gateway';
      setQueryResult({ error: errorMessage });
    }
  };

  const handleRunAIDoctor = async (targetSql?: string) => {
    const queryToDiagnose = targetSql || sql;
    try {
      const res = await explainMutation({ sql: queryToDiagnose }).unwrap();
      setDoctorReport(res);
      setActiveTab('doctor');
    } catch (err) {
      console.error('AI Doctor diagnosis failed:', err);
    }
  };

  const handleStartLoad = async (count: number = 25000) => {
    setIsSimulating(true);
    try {
      await startLoadMutation(count).unwrap();
    } catch (err) {
      console.error('Failed to start load generator:', err);
      setIsSimulating(false);
    }
  };

  const handleStopLoad = async () => {
    setIsSimulating(false);
    try {
      await stopLoadMutation().unwrap();
    } catch (err) {
      console.error('Failed to stop load generator:', err);
    }
  };

  const defaultStats = stats || {
    total_events_received: 0,
    total_events_flushed: 0,
    batches_written: 0,
    current_buffer_size: 0,
    buffer_capacity: 100000,
    avg_flush_latency_ms: 0,
    events_per_second: 0,
    active_workers: 4,
    dropped_events: 0,
  };

  const clickhouseStatus = health?.clickhouse || 'connecting...';
  const redisStatus = health?.redis || 'connecting...';

  return (
    <div className="app-container">
      {/* Top Navbar */}
      <nav className="navbar">
        <div className="nav-brand">
          <div className="cf-logo-badge">
            <Zap size={18} fill="#090d16" />
            <span>FLAREHOUSE</span>
          </div>
          <div>
            <div className="nav-title">FlareHouse Platform Tooling</div>
            <div className="nav-subtitle">Analytics Platform Gateway & Systems Telemetry Engine</div>
          </div>
        </div>

        <div className="system-status">
          <div className="status-pill">
            <span className={`status-dot ${clickhouseStatus === 'healthy' ? '' : 'status-dot-off'}`} />
            <span>ClickHouse: <strong>{clickhouseStatus}</strong></span>
          </div>
          <div className="status-pill">
            <span className={`status-dot ${redisStatus === 'healthy' ? '' : 'status-dot-off'}`} />
            <span>Redis Cache: <strong>{redisStatus}</strong></span>
          </div>
          <div className="status-pill">
            <Layers size={14} color="#f38020" />
            <span>Workers: <strong>{defaultStats.active_workers} Go Daemons</strong></span>
          </div>
        </div>
      </nav>

      {/* Tabs Header */}
      <div className="tabs-header">
        <button
          type="button"
          className={`tab-btn ${activeTab === 'query' ? 'active' : ''}`}
          onClick={() => setActiveTab('query')}
        >
          <Terminal size={16} />
          <span>Analytical Query Console</span>
        </button>

        <button
          type="button"
          className={`tab-btn ${activeTab === 'doctor' ? 'active' : ''}`}
          onClick={() => setActiveTab('doctor')}
        >
          <Sparkles size={16} />
          <span>AI Query Doctor</span>
          {doctorReport && <span className="badge badge-orange">Analysis Ready</span>}
        </button>

        <button
          type="button"
          className={`tab-btn ${activeTab === 'streaming' ? 'active' : ''}`}
          onClick={() => setActiveTab('streaming')}
        >
          <Activity size={16} />
          <span>Streaming Ingestion Engine</span>
          {defaultStats.events_per_second > 0 && (
            <span className="badge badge-green">{defaultStats.events_per_second.toLocaleString()} eps</span>
          )}
        </button>

        <button
          type="button"
          className={`tab-btn ${activeTab === 'partitions' ? 'active' : ''}`}
          onClick={() => {
            setActiveTab('partitions');
            refetchPartitions();
          }}
        >
          <Database size={16} />
          <span>ClickHouse Partitions</span>
        </button>
      </div>

      {/* Main Workspace */}
      <main className="main-content">
        {activeTab === 'query' && (
          <div>
            <QueryEditor
              sql={sql}
              onChange={setSql}
              bypassCache={bypassCache}
              onBypassCacheChange={setBypassCache}
              onRunQuery={() => handleRunQuery()}
              onExplain={() => handleRunAIDoctor()}
              isLoading={isQueryLoading}
              doctorLoading={isDoctorLoading}
              onSelectPreset={(newSql) => setSql(newSql)}
            />

            <GuardrailPanel
              queryResult={queryResult}
              onAskDoctor={() => handleRunAIDoctor()}
            />

            <ResultsTable queryResult={queryResult} />
          </div>
        )}

        {activeTab === 'doctor' && (
          <AIDoctorPanel
            report={doctorReport}
            loading={isDoctorLoading}
            onDiagnose={() => handleRunAIDoctor()}
            onApplyOptimized={(optimizedSql) => {
              setSql(optimizedSql);
              setActiveTab('query');
              handleRunQuery(optimizedSql);
            }}
          />
        )}

        {activeTab === 'streaming' && (
          <IngestStats
            stats={defaultStats}
            isSimulating={isSimulating}
            onStartLoad={handleStartLoad}
            onStopLoad={handleStopLoad}
          />
        )}

        {activeTab === 'partitions' && (
          <PartitionExplorer
            partitions={partitions}
            isLoading={isPartitionsLoading}
            onRefresh={() => refetchPartitions()}
          />
        )}
      </main>
    </div>
  );
}

export default App;
