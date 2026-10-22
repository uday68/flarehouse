package aidoctor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/uday68/flarehouse/internal/model"
)

type Doctor struct {
	geminiAPIKey string
	httpClient   *http.Client
}

func NewDoctor() *Doctor {
	apiKey := os.Getenv("GEMINI_API_KEY")
	return &Doctor{
		geminiAPIKey: apiKey,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

type geminiRequest struct {
	Contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// Diagnose evaluates the query and its EXPLAIN plan, returning actionable optimizations
func (d *Doctor) Diagnose(ctx context.Context, sqlQuery, explainPlan string) model.AIDoctorAnalysis {
	// Attempt real Gemini diagnosis if API key is provided
	if d.geminiAPIKey != "" {
		analysis, err := d.callGemini(ctx, sqlQuery, explainPlan)
		if err == nil && analysis != nil {
			analysis.GeneratedByAI = true
			return *analysis
		}
	}

	// Fallback to ClickHouse Systems Heuristics Engine
	return d.heuristicDiagnose(sqlQuery, explainPlan)
}

func (d *Doctor) callGemini(ctx context.Context, sqlQuery, explainPlan string) (*model.AIDoctorAnalysis, error) {
	prompt := fmt.Sprintf(`You are a Principal Database Engineer at Cloudflare specializing in ClickHouse performance and internal platform tooling.
Analyze the following SQL query and its ClickHouse EXPLAIN output:

SQL Query:
%s

EXPLAIN Output:
%s

Respond ONLY with a valid JSON object matching this schema:
{
  "issues_identified": ["issue 1", "issue 2"],
  "recommendations": ["recommendation 1", "recommendation 2"],
  "optimized_sql": "improved sql query string",
  "expected_impact": "estimated percentage speedup or RAM reduction"
}`, sqlQuery, explainPlan)

	reqBody := geminiRequest{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: prompt},
				},
			},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s", d.geminiAPIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gResp geminiResponse
	if err := json.Unmarshal(body, &gResp); err != nil {
		return nil, err
	}

	if len(gResp.Candidates) == 0 || len(gResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	rawText := gResp.Candidates[0].Content.Parts[0].Text
	rawText = strings.TrimSpace(rawText)
	rawText = strings.TrimPrefix(rawText, "```json")
	rawText = strings.TrimPrefix(rawText, "```")
	rawText = strings.TrimSuffix(rawText, "```")
	rawText = strings.TrimSpace(rawText)

	var analysis model.AIDoctorAnalysis
	if err := json.Unmarshal([]byte(rawText), &analysis); err != nil {
		return nil, err
	}

	analysis.OriginalSQL = sqlQuery
	analysis.ExplainPlan = explainPlan
	return &analysis, nil
}

func (d *Doctor) heuristicDiagnose(sqlQuery, explainPlan string) model.AIDoctorAnalysis {
	lowerSQL := strings.ToLower(sqlQuery)
	issues := []string{}
	recs := []string{}
	impact := "Up to 5x - 15x query latency reduction"
	optimized := sqlQuery

	// Heuristic 1: Full table scan on unoptimized table
	if strings.Contains(lowerSQL, "unoptimized_legacy_logs") {
		issues = append(issues, "Table uses ORDER BY tuple() anti-pattern: ClickHouse has no primary sparse index to skip granule blocks.")
		issues = append(issues, "Partitioning by daily toDate(timestamp) creates excessive small parts under write loads.")
		recs = append(recs, "Migrate dataset to ORDER BY (tenant_id, timestamp) and partition by toYYYYMM(timestamp).")
		recs = append(recs, "Apply LowCardinality(String) to low-entropy attributes to enable ClickHouse dictionary compression.")
		impact = "80% reduction in disk I/O and 10x-20x faster range queries"
		optimized = strings.Replace(sqlQuery, "unoptimized_legacy_logs", "edge_http_logs", 1)
	}

	// Heuristic 2: Scanning raw logs for 1-minute aggregations
	if strings.Contains(lowerSQL, "edge_http_logs") && strings.Contains(lowerSQL, "count()") && strings.Contains(lowerSQL, "group by") {
		issues = append(issues, "Raw Table Aggregation: Scanning millions of raw edge events on-the-fly for metrics rollups.")
		recs = append(recs, "Leverage the pre-aggregated Materialized View 'flarehouse.edge_metrics_1m' powered by SummingMergeTree.")
		impact = "50x faster aggregation; reads ~1,000 pre-aggregated rows instead of raw telemetry"
		optimized = "SELECT minute, tenant_id, edge_colo, sum(request_count) as total_requests, sum(total_bytes) as total_bytes\nFROM flarehouse.edge_metrics_1m\nWHERE minute >= now() - INTERVAL 1 HOUR\nGROUP BY minute, tenant_id, edge_colo\nORDER BY minute DESC\nLIMIT 100;"
	}

	// Heuristic 3: SELECT * anti-pattern
	if strings.Contains(lowerSQL, "select *") {
		issues = append(issues, "Columnar Storage Penalty: 'SELECT *' reads every column file from disk, negating ClickHouse's columnar efficiency.")
		recs = append(recs, "Project only required columns (e.g., SELECT timestamp, client_ip, status_code).")
	}

	// Heuristic 4: Missing Time Filter on Partitioned Data
	if !strings.Contains(lowerSQL, "timestamp") && !strings.Contains(lowerSQL, "date") {
		issues = append(issues, "Missing Partition Filter: ClickHouse cannot perform partition pruning without a date/timestamp condition.")
		recs = append(recs, "Add 'WHERE timestamp >= now() - INTERVAL 24 HOUR' to scan only active monthly partitions.")
	}

	if len(issues) == 0 {
		issues = append(issues, "Query adheres to basic ClickHouse standards.")
		recs = append(recs, "Consider evaluating ClickHouse PROJECTIONS if frequent sub-aggregations on status_code are needed.")
		impact = "Optimal configuration detected."
	}

	return model.AIDoctorAnalysis{
		OriginalSQL:      sqlQuery,
		ExplainPlan:      explainPlan,
		IssuesIdentified: issues,
		Recommendations:  recs,
		OptimizedSQL:     optimized,
		ExpectedImpact:   impact,
		GeneratedByAI:    false,
	}
}
