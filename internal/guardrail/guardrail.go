package guardrail

import (
	"regexp"
	"strings"

	"github.com/uday68/flarehouse/internal/model"
)

var (
	destructiveRegex = regexp.MustCompile(`(?i)\b(DROP|TRUNCATE|ALTER|DETACH|ATTACH|KILL|SYSTEM|CREATE|INSERT)\b`)
	selectStarRegex  = regexp.MustCompile(`(?i)\bSELECT\s+\*\s+FROM\b`)
	limitRegex       = regexp.MustCompile(`(?i)\bLIMIT\s+\d+`)
	groupByRegex     = regexp.MustCompile(`(?i)\bGROUP\s+BY\b`)
	aggRegex         = regexp.MustCompile(`(?i)\b(count|sum|avg|min|max|uniq|any|quantiles?)\s*\(`)
	timeFilterRegex  = regexp.MustCompile(`(?i)\b(timestamp|todate|startofminute|now\(\)|interval)\b`)
	largeScanTables  = []string{"edge_http_logs", "unoptimized_legacy_logs"}
)

type GuardrailEngine struct {
	MaxDefaultLimit   int
	EnforcePartition  bool
	DisallowSelectAll bool
}

func NewGuardrailEngine() *GuardrailEngine {
	return &GuardrailEngine{
		MaxDefaultLimit:   10000,
		EnforcePartition:  true,
		DisallowSelectAll: false,
	}
}

// Evaluate inspects the query and decides if it violates platform guardrails
func (g *GuardrailEngine) Evaluate(rawSQL string) model.GuardrailDecision {
	normalized := strings.TrimSpace(rawSQL)
	lower := strings.ToLower(normalized)

	decision := model.GuardrailDecision{
		Safe:             true,
		ViolationReasons: []string{},
		Warnings:         []string{},
	}

	// 1. Destructive statement check
	if destructiveRegex.MatchString(normalized) {
		decision.Safe = false
		decision.ViolationReasons = append(decision.ViolationReasons,
			"Destructive or mutating DDL/DML statements (DROP, ALTER, TRUNCATE, SYSTEM, INSERT) are blocked on the analytical gateway.")
		return decision
	}

	// Must begin with SELECT, WITH, or EXPLAIN
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") && !strings.HasPrefix(lower, "explain") {
		decision.Safe = false
		decision.ViolationReasons = append(decision.ViolationReasons, "Gateway only accepts analytical queries beginning with SELECT, WITH, or EXPLAIN.")
		return decision
	}

	// Check if querying wide telemetry tables
	targetsBigTable := false
	for _, t := range largeScanTables {
		if strings.Contains(lower, t) {
			targetsBigTable = true
			break
		}
	}

	// 2. Anti-pattern: SELECT * on columnar tables
	if selectStarRegex.MatchString(normalized) {
		decision.Warnings = append(decision.Warnings,
			"Columnar Anti-Pattern: 'SELECT *' forces ClickHouse to decompress and read all columns from disk. Explicitly specify column projections for optimal performance.")
	}

	// 3. Partition Pruning check
	hasPartitionFilter := timeFilterRegex.MatchString(lower)
	decision.HasPartitionFilter = hasPartitionFilter

	if targetsBigTable && !hasPartitionFilter {
		decision.Warnings = append(decision.Warnings,
			"Unbounded Partition Scan: Query references telemetry table without filtering by timestamp/date. This will trigger a full partition scan across all storage parts.")
	}

	// 4. Missing LIMIT check on raw row queries
	hasLimit := limitRegex.MatchString(lower)
	hasGroupBy := groupByRegex.MatchString(lower)
	hasAgg := aggRegex.MatchString(lower)
	decision.HasLimit = hasLimit

	if !hasLimit && !hasGroupBy && !hasAgg && targetsBigTable {
		decision.Safe = false
		decision.ViolationReasons = append(decision.ViolationReasons,
			"Missing LIMIT: Raw scan query on telemetry dataset lacks a LIMIT clause. Add 'LIMIT 1000' or an aggregation to protect client memory buffers.")
	}

	return decision
}
