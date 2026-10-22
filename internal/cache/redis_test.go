package cache

import (
	"testing"
)

func TestHashQueryNormalization(t *testing.T) {
	q1 := "SELECT client_ip, host FROM flarehouse.edge_http_logs WHERE status_code = 200 LIMIT 10"
	q2 := "  SELECT   client_ip,  host  FROM  flarehouse.edge_http_logs  WHERE  status_code = 200  LIMIT 10  "
	q3 := "select client_ip, host from flarehouse.edge_http_logs where status_code = 200 limit 10"

	h1 := HashQuery(q1)
	h2 := HashQuery(q2)
	h3 := HashQuery(q3)

	if h1 != h2 {
		t.Fatalf("expected whitespace normalization to yield identical hashes, got %s and %s", h1, h2)
	}

	if h1 != h3 {
		t.Fatalf("expected case normalization to yield identical hashes, got %s and %s", h1, h3)
	}
}
