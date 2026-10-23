import { HardDrive, RefreshCw } from 'lucide-react';
import type { PartitionInfo } from '../types';

interface PartitionExplorerProps {
  partitions: PartitionInfo[];
  isLoading: boolean;
  onRefresh: () => void;
}

export function PartitionExplorer({ partitions, isLoading, onRefresh }: PartitionExplorerProps) {
  return (
    <div className="card">
      <div className="card-header">
        <div className="card-title">
          <HardDrive size={18} color="#f38020" />
          <span>ClickHouse Storage & Partition Pruning Telemetry</span>
        </div>
        <button
          type="button"
          className="btn btn-secondary"
          onClick={onRefresh}
          disabled={isLoading}
        >
          <RefreshCw size={14} className={isLoading ? 'spin' : ''} />
          <span>{isLoading ? 'Refreshing...' : 'Refresh'}</span>
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
  );
}
