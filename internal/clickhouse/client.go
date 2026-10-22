package clickhouse

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/uday68/flarehouse/internal/model"
)

type Client struct {
	conn driver.Conn
}

func NewClient(addr, database, username, password string) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:     5 * time.Second,
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 10 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize clickhouse connection: %w", err)
	}

	return &Client{conn: conn}, nil
}

// Ping checks connectivity
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

// Close closes the underlying pool
func (c *Client) Close() error {
	return c.conn.Close()
}

// InsertBatch flushes a slice of telemetry events to ClickHouse using native columnar batch
func (c *Client) InsertBatch(ctx context.Context, events []model.TelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO flarehouse.edge_http_logs")
	if err != nil {
		return fmt.Errorf("prepare batch error: %w", err)
	}

	for _, e := range events {
		reqUUID, err := uuid.Parse(e.RequestID)
		if err != nil {
			reqUUID = uuid.New()
		}

		err = batch.Append(
			e.Timestamp,
			e.TenantID,
			reqUUID,
			e.ClientIP,
			e.Host,
			e.Path,
			e.Method,
			e.StatusCode,
			e.ResponseBytes,
			e.DurationMs,
			e.EdgeColo,
			e.Country,
			e.CacheStatus,
			e.ThreatScore,
		)
		if err != nil {
			return fmt.Errorf("batch append error: %w", err)
		}
	}

	return batch.Send()
}

// ExecuteQuery executes a read-only analytical SQL query and returns formatted tabular data
func (c *Client) ExecuteQuery(ctx context.Context, sqlQuery string) (*model.QueryResponse, error) {
	start := time.Now()

	rows, err := c.conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("clickhouse query execution error: %w", err)
	}
	defer rows.Close()

	cols := rows.Columns()
	types := rows.ColumnTypes()

	var data [][]interface{}

	for rows.Next() {
		scanArgs := make([]interface{}, len(cols))
		for i, colType := range types {
			scanType := colType.ScanType()
			if scanType != nil {
				scanArgs[i] = reflect.New(scanType).Interface()
			} else {
				var s string
				scanArgs[i] = &s
			}
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("row scan error: %w", err)
		}

		// Convert bytes or special types to JSON-serializable primitives
		cleanedRow := make([]interface{}, len(cols))
		for i := range scanArgs {
			val := reflect.ValueOf(scanArgs[i]).Elem().Interface()
			if b, ok := val.([]byte); ok {
				cleanedRow[i] = string(b)
			} else if t, ok := val.(time.Time); ok {
				cleanedRow[i] = t.Format(time.RFC3339Nano)
			} else {
				cleanedRow[i] = val
			}
		}
		data = append(data, cleanedRow)
	}

	duration := time.Since(start)

	_ = types // column types can be used for metadata

	return &model.QueryResponse{
		Columns:       cols,
		Data:          data,
		Rows:          len(data),
		DurationMs:    float64(duration.Microseconds()) / 1000.0,
		ExecutionTime: duration.Seconds(),
	}, nil
}

// ExplainQuery runs ClickHouse EXPLAIN on the given SQL
func (c *Client) ExplainQuery(ctx context.Context, sqlQuery string) (string, error) {
	trimmed := strings.TrimSpace(sqlQuery)
	explainSQL := fmt.Sprintf("EXPLAIN indexes = 1, description = 1 %s", trimmed)

	rows, err := c.conn.Query(ctx, explainSQL)
	if err != nil {
		// Fallback to simpler explain if indexes=1 fails on certain queries
		rows, err = c.conn.Query(ctx, fmt.Sprintf("EXPLAIN %s", trimmed))
		if err != nil {
			return "", fmt.Errorf("explain query failed: %w", err)
		}
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err == nil {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n"), nil
}

// GetTablePartitions returns partition breakdown from system.parts
func (c *Client) GetTablePartitions(ctx context.Context, tableName string) ([]model.TablePartitionInfo, error) {
	q := `
		SELECT 
			database,
			table,
			partition,
			name,
			part_type,
			rows,
			bytes_on_disk,
			data_uncompressed_bytes
		FROM system.parts
		WHERE database = 'flarehouse'
		AND active = 1
		ORDER BY table, partition DESC
		LIMIT 50
	`
	rows, err := c.conn.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("system.parts query failed: %w", err)
	}
	defer rows.Close()

	var results []model.TablePartitionInfo
	for rows.Next() {
		var p model.TablePartitionInfo
		var uncomp uint64
		if err := rows.Scan(&p.Database, &p.Table, &p.Partition, &p.Name, &p.PartType, &p.Rows, &p.BytesOnDisk, &uncomp); err != nil {
			continue
		}
		p.DataUncomp = uncomp
		if uncomp > 0 {
			p.CompressRatio = float64(uncomp) / float64(p.BytesOnDisk)
		}
		results = append(results, p)
	}

	return results, nil
}
