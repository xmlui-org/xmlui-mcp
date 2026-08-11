package server

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func useTestAnalytics(t *testing.T) *Analytics {
	t.Helper()
	previous := globalAnalytics
	analytics := newAnalytics(filepath.Join(t.TempDir(), "analytics.jsonl"))
	globalAnalytics = analytics
	t.Cleanup(func() {
		globalAnalytics = previous
	})
	return analytics
}

func searchRequest(query string) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = map[string]interface{}{"query": query}
	return req
}

func boolValue(t *testing.T, value *bool) bool {
	t.Helper()
	if value == nil {
		t.Fatal("expected non-nil bool")
	}
	return *value
}

func intValue(t *testing.T, value *int) int {
	t.Helper()
	if value == nil {
		t.Fatal("expected non-nil int")
	}
	return *value
}

func syntheticSearchHandler(toolName, query, human string, summary MediatorJSON, executionSuccess bool) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		recordSearchObservation(ctx, toolName, query, executionSuccess, summary, "xmlui@test")
		if !executionSuccess {
			return nil, errors.New("mediator failed")
		}
		return mcp.NewToolResultText(human), nil
	}
}

func TestSearchAnalyticsNoMatchIgnoresGuidanceText(t *testing.T) {
	analytics := useTestAnalytics(t)
	summary := MediatorJSON{
		Sections:   map[string][]resultItem{"howtos": {}},
		Facets:     map[string]FacetCounts{"howtos": {}},
		Confidence: "low",
		Tokens: map[string][]string{
			"kept":     {"slider", "handles"},
			"removed":  {},
			"expanded": {"slider", "handles"},
		},
	}
	human := "No matches found.\n\nTry another tool.\n---\nPreferred tool: xmlui_search\n"
	handler := WithSearchAnalytics("xmlui_search_howto", syntheticSearchHandler("xmlui_search_howto", "slider handles", human, summary, true))

	if _, err := handler(context.Background(), searchRequest("slider handles")); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if len(analytics.data.SearchQueries) != 1 {
		t.Fatalf("got %d search records, want 1", len(analytics.data.SearchQueries))
	}
	record := analytics.data.SearchQueries[0]
	if record.SchemaVersion != 2 || !boolValue(t, record.ExecutionSuccess) || boolValue(t, record.YieldedResults) {
		t.Fatalf("unexpected outcomes: %+v", record)
	}
	if got := intValue(t, record.MatchedFileCount); got != 0 {
		t.Fatalf("matched_file_count = %d, want 0", got)
	}
	if got := intValue(t, record.MatchedLineCount); got != 0 {
		t.Fatalf("matched_line_count = %d, want 0", got)
	}
	if record.MatchedPaths == nil || len(record.MatchedPaths) != 0 {
		t.Fatalf("matched_paths = %#v, want []", record.MatchedPaths)
	}
	if record.ResultCount != 0 || record.Success || record.FoundURLs != nil {
		t.Fatalf("legacy fields became authoritative: %+v", record)
	}
}

func TestSearchAnalyticsHitUsesExactMediatorMetrics(t *testing.T) {
	analytics := useTestAnalytics(t)
	summary := MediatorJSON{
		Sections: map[string][]resultItem{
			"howtos": {
				{Path: "docs/howto/range.md", Line: 4, Score: 8},
				{Path: "docs/howto/range.md", Line: 8, Score: 8},
				{Path: "docs/howto/input.md", Line: 2, Score: 3},
			},
		},
		Facets:     map[string]FacetCounts{"howtos": {Files: 2, Matches: 3}},
		Confidence: "high",
		Tokens: map[string][]string{
			"kept":     {"range", "slider"},
			"removed":  {"a"},
			"expanded": {"range", "slider"},
		},
	}
	handler := WithSearchAnalytics("xmlui_search_howto", syntheticSearchHandler("xmlui_search_howto", "a range slider", "arbitrary output", summary, true))

	if _, err := handler(context.Background(), searchRequest("a range slider")); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	record := analytics.data.SearchQueries[0]
	if !boolValue(t, record.YieldedResults) {
		t.Fatal("yielded_results = false, want true")
	}
	if got := intValue(t, record.MatchedFileCount); got != 2 {
		t.Fatalf("matched_file_count = %d, want 2", got)
	}
	if got := intValue(t, record.MatchedLineCount); got != 3 {
		t.Fatalf("matched_line_count = %d, want 3", got)
	}
	wantPaths := []string{"docs/howto/range.md", "docs/howto/input.md"}
	if strings.Join(record.MatchedPaths, "|") != strings.Join(wantPaths, "|") {
		t.Fatalf("matched_paths = %#v, want %#v", record.MatchedPaths, wantPaths)
	}
	if record.Confidence != "high" || record.CorpusVersion != "xmlui@test" {
		t.Fatalf("structured metadata not retained: %+v", record)
	}
}

func TestHumanFormattingDoesNotChangeSearchMetrics(t *testing.T) {
	analytics := useTestAnalytics(t)
	summary := MediatorJSON{
		Sections: map[string][]resultItem{"examples": {{Path: "app/App.xmlui", Line: 3, Score: 4}}},
		Facets:   map[string]FacetCounts{"examples": {Files: 1, Matches: 1}},
		Tokens:   map[string][]string{"kept": {"button"}},
	}

	for _, human := range []string{"app/App.xmlui:3: old format", "# Completely different Markdown\n\nGuidance only follows."} {
		handler := WithSearchAnalytics("xmlui_examples", syntheticSearchHandler("xmlui_examples", "button", human, summary, true))
		if _, err := handler(context.Background(), searchRequest("button")); err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
	}

	first, second := analytics.data.SearchQueries[0], analytics.data.SearchQueries[1]
	if intValue(t, first.MatchedFileCount) != intValue(t, second.MatchedFileCount) ||
		intValue(t, first.MatchedLineCount) != intValue(t, second.MatchedLineCount) ||
		strings.Join(first.MatchedPaths, "|") != strings.Join(second.MatchedPaths, "|") {
		t.Fatalf("formatting changed metrics: first=%+v second=%+v", first, second)
	}
}

func TestSearchRecordsShareInvocationID(t *testing.T) {
	analytics := useTestAnalytics(t)
	summary := MediatorJSON{Sections: map[string][]resultItem{}, Facets: map[string]FacetCounts{}, Tokens: map[string][]string{}}
	handler := WithSearchAnalytics("xmlui_search", syntheticSearchHandler("xmlui_search", "query", "No matches found.", summary, true))

	if _, err := handler(context.Background(), searchRequest("query")); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	invocationID := analytics.data.ToolInvocations[0].InvocationID
	if invocationID == "" || analytics.data.SearchQueries[0].InvocationID != invocationID {
		t.Fatalf("invocation IDs do not correlate: tool=%q search=%q", invocationID, analytics.data.SearchQueries[0].InvocationID)
	}
}

func TestOperationalFailureDiffersFromValidMiss(t *testing.T) {
	analytics := useTestAnalytics(t)
	emptySummary := MediatorJSON{Sections: map[string][]resultItem{}, Facets: map[string]FacetCounts{}, Tokens: map[string][]string{}}
	validMiss := WithSearchAnalytics("xmlui_search", syntheticSearchHandler("xmlui_search", "valid", "No matches found.", emptySummary, true))
	operationalFailure := WithSearchAnalytics("xmlui_search", syntheticSearchHandler("xmlui_search", "broken", "", emptySummary, false))
	invalidInput := WithSearchAnalytics("xmlui_search_howto", func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError("Missing or invalid 'query' parameter"), nil
	})

	_, _ = validMiss(context.Background(), searchRequest("valid"))
	_, _ = operationalFailure(context.Background(), searchRequest("broken"))
	_, _ = invalidInput(context.Background(), searchRequest(" "))

	if !boolValue(t, analytics.data.SearchQueries[0].ExecutionSuccess) || boolValue(t, analytics.data.SearchQueries[0].YieldedResults) {
		t.Fatalf("valid miss was not classified correctly: %+v", analytics.data.SearchQueries[0])
	}
	for _, record := range analytics.data.SearchQueries[1:] {
		if boolValue(t, record.ExecutionSuccess) {
			t.Fatalf("operational failure classified as successful: %+v", record)
		}
	}
}

func TestLegacyJSONLLoadsAndMixedSummaryUsesOnlyV2Rates(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "analytics.jsonl")
	legacy := `{"type":"search_query","timestamp":"2025-01-01T00:00:00Z","tool_name":"xmlui_search_howto","query":"legacy","result_count":7,"success":true,"found_urls":["old.md"]}` + "\n"
	if err := os.WriteFile(logPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	analytics := newAnalytics(logPath)
	if len(analytics.data.SearchQueries) != 1 || analytics.data.SearchQueries[0].SchemaVersion != 0 {
		t.Fatalf("legacy record failed to load: %+v", analytics.data.SearchQueries)
	}

	addV2 := func(executionSuccess, yieldedResults bool) {
		files, lines := 0, 0
		analytics.data.SearchQueries = append(analytics.data.SearchQueries, SearchQuery{
			SchemaVersion:    2,
			ExecutionSuccess: &executionSuccess,
			YieldedResults:   &yieldedResults,
			MatchedFileCount: &files,
			MatchedLineCount: &lines,
		})
	}
	addV2(true, true)
	addV2(true, false)
	addV2(false, false)

	summary := analytics.GetSummary()
	if summary["legacy_search_query_count"] != 1 || summary["search_v2_query_count"] != 3 {
		t.Fatalf("mixed record counts are wrong: %#v", summary)
	}
	if got := summary["search_execution_success_rate"].(float64); math.Abs(got-66.6666666667) > 0.0001 {
		t.Fatalf("execution success rate = %f, want 66.67", got)
	}
	if got := summary["search_retrieval_hit_rate"].(float64); got != 50 {
		t.Fatalf("retrieval hit rate = %f, want 50", got)
	}
	if summary["search_zero_result_count"] != 1 {
		t.Fatalf("zero-result count = %#v, want 1", summary["search_zero_result_count"])
	}
}

func TestAllMediatedSearchToolsUseStructuredAnalytics(t *testing.T) {
	analytics := useTestAnalytics(t)
	root := t.TempDir()
	manifest := `{"componentDocs":"components","componentSource":"source","extensionDocs":"components","extensionSource":"packages","pages":"pages","howto":"howto","blog":"blog"}`
	if err := os.WriteFile(filepath.Join(root, "mcp-paths.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"components", "source", "packages", "pages", "howto", "blog", "examples"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "components", "needle.mdx"), []byte("# Unique needle"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "howto", "needle.md"), []byte("# Unique needle"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "examples", "needle.xmlui"), []byte("<Text>Unique needle</Text>"), 0o600); err != nil {
		t.Fatal(err)
	}

	ResetRepoPaths()
	_, searchHandler := NewSearchTool(root, nil)
	_, howtoHandler := NewSearchHowtoTool(root)
	_, examplesHandler := NewExamplesTool([]string{filepath.Join(root, "examples")})
	wrapped := []struct {
		name    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{"xmlui_search", WithSearchAnalytics("xmlui_search", searchHandler)},
		{"xmlui_search_howto", WithSearchAnalytics("xmlui_search_howto", howtoHandler)},
		{"xmlui_examples", WithSearchAnalytics("xmlui_examples", examplesHandler)},
	}
	for _, item := range wrapped {
		if _, err := item.handler(context.Background(), searchRequest("unique needle")); err != nil {
			t.Fatalf("%s returned error: %v", item.name, err)
		}
	}

	if len(analytics.data.SearchQueries) != len(wrapped) {
		t.Fatalf("got %d search records, want %d", len(analytics.data.SearchQueries), len(wrapped))
	}
	for i, record := range analytics.data.SearchQueries {
		if record.ToolName != wrapped[i].name || record.SchemaVersion != 2 || !boolValue(t, record.ExecutionSuccess) || !boolValue(t, record.YieldedResults) {
			t.Fatalf("tool %s did not use structured analytics: %+v", wrapped[i].name, record)
		}
	}
}

func TestV2JSONIncludesExplicitZeroValues(t *testing.T) {
	analytics := useTestAnalytics(t)
	emptySummary := MediatorJSON{Sections: map[string][]resultItem{}, Facets: map[string]FacetCounts{}, Tokens: map[string][]string{}}
	handler := WithSearchAnalytics("xmlui_search", syntheticSearchHandler("xmlui_search", "miss", "many\nlines\nof guidance", emptySummary, true))
	_, _ = handler(context.Background(), searchRequest("miss"))

	data, err := os.ReadFile(analytics.logFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record["type"] != "search_query" {
			continue
		}
		if record["execution_success"] != true || record["yielded_results"] != false || record["matched_file_count"] != float64(0) || record["matched_line_count"] != float64(0) {
			t.Fatalf("explicit zero-value contract missing: %s", line)
		}
		if paths, ok := record["matched_paths"].([]interface{}); !ok || len(paths) != 0 {
			t.Fatalf("matched_paths is not an explicit empty array: %s", line)
		}
		if _, exists := record["success"]; exists {
			t.Fatalf("deprecated success field emitted in v2: %s", line)
		}
	}
}

func TestSearchAnalyticsRecordsGapClassificationSignals(t *testing.T) {
	analytics := useTestAnalytics(t)
	summary := MediatorJSON{
		Sections: map[string][]resultItem{
			"howtos": {
				{Path: "howto/format-values.md", Score: 4.5, TitleMatch: true},
				{Path: "howto/format-values.md", Score: 4.5, TitleMatch: true},
				{Path: "howto/style-text.md", Score: 3.0},
			},
		},
		Facets:     map[string]FacetCounts{"howtos": {Files: 2, Matches: 3}},
		Confidence: "medium",
		Tokens: map[string][]string{
			"kept":     {"relative", "timestamp"},
			"removed":  {},
			"expanded": {"relative", "timestamp"},
		},
	}
	human := "Query: ...\n## howto/format-values.md\n"
	handler := WithSearchAnalytics("xmlui_search_howto", syntheticSearchHandler("xmlui_search_howto", "relative timestamp", human, summary, true))

	if _, err := handler(context.Background(), searchRequest("relative timestamp")); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if len(analytics.data.SearchQueries) != 1 {
		t.Fatalf("got %d search records, want 1", len(analytics.data.SearchQueries))
	}
	record := analytics.data.SearchQueries[0]
	if record.TopScore == nil || *record.TopScore != 4.5 {
		t.Fatalf("top_score = %v, want 4.5", record.TopScore)
	}
	if record.ScoreGap == nil || math.Abs(*record.ScoreGap-1.5) > 1e-9 {
		t.Fatalf("score_gap = %v, want 1.5", record.ScoreGap)
	}
	if got := intValue(t, record.TitleMatchCount); got != 1 {
		t.Fatalf("title_match_count = %d, want 1", got)
	}
}

func TestSearchAnalyticsGapSignalsOmittedOnMiss(t *testing.T) {
	analytics := useTestAnalytics(t)
	summary := MediatorJSON{
		Sections:   map[string][]resultItem{"howtos": {}},
		Facets:     map[string]FacetCounts{"howtos": {}},
		Confidence: "low",
		Tokens:     map[string][]string{"kept": {}, "removed": {}, "expanded": {}},
	}
	handler := WithSearchAnalytics("xmlui_search_howto", syntheticSearchHandler("xmlui_search_howto", "voice input", "No matches found.\n", summary, true))

	if _, err := handler(context.Background(), searchRequest("voice input")); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	record := analytics.data.SearchQueries[0]
	if record.TopScore != nil || record.ScoreGap != nil {
		t.Fatalf("expected omitted score signals on miss, got top=%v gap=%v", record.TopScore, record.ScoreGap)
	}
	if got := intValue(t, record.TitleMatchCount); got != 0 {
		t.Fatalf("title_match_count = %d, want 0", got)
	}
}
