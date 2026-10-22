package aidoctor

import (
	"context"
	"strings"
	"testing"
)

func TestHeuristicDiagnosis(t *testing.T) {
	doc := NewDoctor()

	// 1. Test unoptimized legacy table diagnosis
	unoptimizedSQL := "SELECT * FROM flarehouse.unoptimized_legacy_logs WHERE url_blob LIKE '%login%'"
	analysis := doc.Diagnose(context.Background(), unoptimizedSQL, "Read 100000 parts, Filter: url_blob LIKE '%login%'")

	if len(analysis.IssuesIdentified) == 0 {
		t.Fatal("expected issues identified for unoptimized table, got 0")
	}

	foundTupleIssue := false
	for _, issue := range analysis.IssuesIdentified {
		if strings.Contains(issue, "tuple()") {
			foundTupleIssue = true
			break
		}
	}

	if !foundTupleIssue {
		t.Errorf("expected diagnosis to flag 'tuple()' anti-pattern, got: %v", analysis.IssuesIdentified)
	}

	if analysis.OptimizedSQL == "" {
		t.Error("expected suggested optimized SQL")
	}
}
