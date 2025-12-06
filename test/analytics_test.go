package test

import (
	"log/slog"
	"testing"

	"github.com/mikeschinkel/go-dt"
	"github.com/mikeschinkel/go-testutil"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/mcppkg"
)

// bufferedJSONHandlerForAnalytics implements FilepathGetter for analytics tests
type bufferedJSONHandlerForAnalytics struct {
	slog.Handler
	filepath dt.Filepath
}

func (h bufferedJSONHandlerForAnalytics) Filepath() dt.Filepath {
	return h.filepath
}

func getTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	tempDir := dt.Filepath(t.TempDir())
	return slog.New(bufferedJSONHandlerForAnalytics{
		Handler:  testutil.NewBufferedLogHandler(),
		filepath: dt.FilepathJoin(tempDir, "test-analytics.json"),
	})
}

func TestGetAnalyticsSummary_WhenGlobalAnalyticsIsNil(t *testing.T) {
	// GetAnalyticsSummary should return an empty map when globalAnalytics is nil
	summary := mcppkg.GetAnalyticsSummary()

	if summary == nil {
		t.Fatal("GetAnalyticsSummary() should not return nil")
	}

	if len(summary) != 0 {
		t.Errorf("Expected empty summary when globalAnalytics is nil, got %d entries", len(summary))
	}
}

func TestGetAnalyticsSummary_WithEmptyData(t *testing.T) {
	// Initialize analytics with empty data
	logger := getTestLogger(t)
	err := mcppkg.InitializeAnalytics(logger)
	if err != nil {
		t.Fatalf("Failed to initialize analytics: %v", err)
	}

	summary := mcppkg.GetAnalyticsSummary()

	if summary == nil {
		t.Fatal("GetAnalyticsSummary() should not return nil")
	}

	// Should have structure even with no data
	if _, ok := summary["total_tool_invocations"]; !ok {
		t.Error("Summary should have 'total_tool_invocations' key")
	}

	if _, ok := summary["total_search_queries"]; !ok {
		t.Error("Summary should have 'total_search_queries' key")
	}

	// Check that totals are zero
	if summary["total_tool_invocations"] != 0 {
		t.Errorf("Expected 0 tool invocations, got %v", summary["total_tool_invocations"])
	}

	if summary["total_search_queries"] != 0 {
		t.Errorf("Expected 0 search queries, got %v", summary["total_search_queries"])
	}
}

func TestAnalytics_GetSummary_WithToolInvocations(t *testing.T) {
	logger := getTestLogger(t)
	analytics := mcppkg.NewAnalytics(logger)

	// Log some tool invocations
	analytics.LogToolInvocation("xmlui_search", map[string]interface{}{"query": "Button"}, true, 1234, "")
	analytics.LogToolInvocation("xmlui_search", map[string]interface{}{"query": "Avatar"}, true, 567, "")
	analytics.LogToolInvocation("xmlui_read_file", map[string]interface{}{"path": "Button.md"}, true, 5000, "")
	analytics.LogToolInvocation("xmlui_search", map[string]interface{}{"query": "NoMatch"}, false, 0, "not found")

	summary := analytics.GetSummary()

	// Check total invocations
	if summary["total_tool_invocations"] != 4 {
		t.Errorf("Expected 4 total invocations, got %v", summary["total_tool_invocations"])
	}

	// Check tool counts
	toolCounts, ok := summary["tool_usage_counts"].(map[string]int)
	if !ok {
		t.Fatal("tool_usage_counts should be a map[string]int")
	}

	if toolCounts["xmlui_search"] != 3 {
		t.Errorf("Expected 3 xmlui_search invocations, got %d", toolCounts["xmlui_search"])
	}

	if toolCounts["xmlui_read_file"] != 1 {
		t.Errorf("Expected 1 xmlui_read_file invocation, got %d", toolCounts["xmlui_read_file"])
	}

	// Check success rates
	successRates, ok := summary["tool_success_rates"].(map[string]float64)
	if !ok {
		t.Fatal("tool_success_rates should be a map[string]float64")
	}

	// xmlui_search: 2 successes out of 3 = 66.67%
	expectedRate := 200.0 / 3.0
	if successRates["xmlui_search"] < expectedRate-0.1 || successRates["xmlui_search"] > expectedRate+0.1 {
		t.Errorf("Expected xmlui_search success rate ~66.67%%, got %.2f%%", successRates["xmlui_search"])
	}

	// xmlui_read_file: 1 success out of 1 = 100%
	if successRates["xmlui_read_file"] != 100.0 {
		t.Errorf("Expected xmlui_read_file success rate 100%%, got %.2f%%", successRates["xmlui_read_file"])
	}

	// Check average result sizes
	avgSizes, ok := summary["tool_avg_result_sizes"].(map[string]int)
	if !ok {
		t.Fatal("tool_avg_result_sizes should be a map[string]int")
	}

	// xmlui_search avg: (1234 + 567 + 0) / 3 = 600
	if avgSizes["xmlui_search"] != 600 {
		t.Errorf("Expected xmlui_search avg result size 600, got %d", avgSizes["xmlui_search"])
	}

	// xmlui_read_file avg: 5000 / 1 = 5000
	if avgSizes["xmlui_read_file"] != 5000 {
		t.Errorf("Expected xmlui_read_file avg result size 5000, got %d", avgSizes["xmlui_read_file"])
	}
}

func TestAnalytics_GetSummary_WithSearchQueries(t *testing.T) {
	logger := getTestLogger(t)
	analytics := mcppkg.NewAnalytics(logger)

	// Log some search queries
	analytics.LogSearchQuery("xmlui_search", "Button", 5, true, []string{"docs/content/components"}, []string{"Button.md"})
	analytics.LogSearchQuery("xmlui_search", "Avatar", 3, true, []string{"docs/content/components"}, []string{"Avatar.md"})
	analytics.LogSearchQuery("xmlui_search", "Button", 5, true, []string{"docs/content/components"}, []string{"Button.md"})
	analytics.LogSearchQuery("xmlui_search", "NoMatch", 0, false, []string{"docs/content/components"}, nil)

	summary := analytics.GetSummary()

	// Check total search queries
	if summary["total_search_queries"] != 4 {
		t.Errorf("Expected 4 total search queries, got %v", summary["total_search_queries"])
	}

	// Check search success rate: 3 successes out of 4 = 75%
	searchSuccessRate, ok := summary["search_success_rate"].(float64)
	if !ok {
		t.Fatal("search_success_rate should be a float64")
	}

	if searchSuccessRate != 75.0 {
		t.Errorf("Expected search success rate 75%%, got %.2f%%", searchSuccessRate)
	}

	// Check popular search terms
	searchTerms, ok := summary["popular_search_terms"].(map[string]int)
	if !ok {
		t.Fatal("popular_search_terms should be a map[string]int")
	}

	if searchTerms["Button"] != 2 {
		t.Errorf("Expected 'Button' to be searched 2 times, got %d", searchTerms["Button"])
	}

	if searchTerms["Avatar"] != 1 {
		t.Errorf("Expected 'Avatar' to be searched 1 time, got %d", searchTerms["Avatar"])
	}

	if searchTerms["NoMatch"] != 1 {
		t.Errorf("Expected 'NoMatch' to be searched 1 time, got %d", searchTerms["NoMatch"])
	}
}

func TestAnalytics_GetSummary_WithMixedData(t *testing.T) {
	logger := getTestLogger(t)
	analytics := mcppkg.NewAnalytics(logger)

	// Log both tool invocations and search queries
	analytics.LogToolInvocation("xmlui_search", map[string]interface{}{"query": "Button"}, true, 1234, "")
	analytics.LogToolInvocation("xmlui_read_file", map[string]interface{}{"path": "Button.md"}, true, 5000, "")
	analytics.LogSearchQuery("xmlui_search", "Button", 5, true, []string{"docs"}, []string{"Button.md"})
	analytics.LogSearchQuery("xmlui_search", "Avatar", 3, true, []string{"docs"}, []string{"Avatar.md"})

	summary := analytics.GetSummary()

	// Check both totals
	if summary["total_tool_invocations"] != 2 {
		t.Errorf("Expected 2 tool invocations, got %v", summary["total_tool_invocations"])
	}

	if summary["total_search_queries"] != 2 {
		t.Errorf("Expected 2 search queries, got %v", summary["total_search_queries"])
	}

	// Verify all expected keys are present
	expectedKeys := []string{
		"total_tool_invocations",
		"total_search_queries",
		"tool_usage_counts",
		"tool_success_rates",
		"tool_avg_result_sizes",
		"search_success_rate",
		"popular_search_terms",
	}

	for _, key := range expectedKeys {
		if _, ok := summary[key]; !ok {
			t.Errorf("Summary should contain key '%s'", key)
		}
	}
}
