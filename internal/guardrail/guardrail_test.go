package guardrail

import (
	"testing"
)

func TestGuardrails(t *testing.T) {
	engine := NewGuardrailEngine()

	tests := []struct {
		name       string
		query      string
		expectSafe bool
	}{
		{
			name:       "Destructive DROP statement",
			query:      "DROP TABLE flarehouse.edge_http_logs",
			expectSafe: false,
		},
		{
			name:       "Destructive TRUNCATE statement",
			query:      "TRUNCATE TABLE flarehouse.edge_http_logs",
			expectSafe: false,
		},
		{
			name:       "Unbounded raw scan missing LIMIT",
			query:      "SELECT client_ip, host FROM flarehouse.edge_http_logs WHERE status_code = 500",
			expectSafe: false,
		},
		{
			name:       "Safe aggregated query with time filter",
			query:      "SELECT count() FROM flarehouse.edge_http_logs WHERE timestamp >= now() - INTERVAL 1 HOUR",
			expectSafe: true,
		},
		{
			name:       "Safe bounded raw query",
			query:      "SELECT client_ip, host FROM flarehouse.edge_http_logs WHERE timestamp >= now() - INTERVAL 1 HOUR LIMIT 50",
			expectSafe: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision := engine.Evaluate(tc.query)
			if decision.Safe != tc.expectSafe {
				t.Errorf("expected safe=%v, got safe=%v (violations: %v)", tc.expectSafe, decision.Safe, decision.ViolationReasons)
			}
		})
	}
}
