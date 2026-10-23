export interface QueryRequest {
  sql: string;
  bypass_cache?: boolean;
}

export interface QueryResponse {
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

export interface AIDoctorAnalysis {
  original_sql: string;
  explain_plan: string;
  issues_identified: string[];
  recommendations: string[];
  optimized_sql: string;
  expected_impact: string;
  generated_by_ai: boolean;
}

export interface IngestStatsData {
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

export interface PartitionInfo {
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

export interface HealthStatus {
  status: string;
  service: string;
  clickhouse: string;
  redis: string;
  timestamp: string;
}

export interface Preset {
  name: string;
  desc: string;
  sql: string;
}
