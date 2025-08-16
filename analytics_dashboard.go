package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewAnalyticsDashboardTool() (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("xmlui_analytics_dashboard",
		mcp.WithDescription("Displays analytics dashboard showing agent usage patterns and trends"),
	)

	tool.InputSchema = mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Output format: 'summary' (default), 'detailed', or 'export'",
				"default":     "summary",
			},
			"period": map[string]interface{}{
				"type":        "string",
				"description": "Time period filter: 'all' (default), 'today', 'week', 'month'",
				"default":     "all",
			},
		},
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		format := "summary"
		period := "all"

		if req.Params.Arguments != nil {
			if f, ok := req.Params.Arguments["format"].(string); ok && f != "" {
				format = f
			}
			if p, ok := req.Params.Arguments["period"].(string); ok && p != "" {
				period = p
			}
		}

		switch format {
		case "export":
			data := ExportAnalyticsData()
			return mcp.NewToolResultText(data), nil
		case "detailed":
			return generateDetailedReport(period)
		default:
			return generateSummaryReport(period)
		}
	}

	return tool, handler
}

func generateSummaryReport(period string) (*mcp.CallToolResult, error) {
	summary := GetAnalyticsSummary()
	
	var out strings.Builder
	out.WriteString("# XMLUI-MCP Usage Analytics Summary\n\n")
	
	// Overall stats
	out.WriteString("## Overall Statistics\n\n")
	if totalInvocations, ok := summary["total_tool_invocations"].(int); ok {
		out.WriteString(fmt.Sprintf("- **Total Tool Invocations**: %d\n", totalInvocations))
	}
	if totalSearches, ok := summary["total_search_queries"].(int); ok {
		out.WriteString(fmt.Sprintf("- **Total Search Queries**: %d\n", totalSearches))
	}
	if activeSessions, ok := summary["active_sessions"].(int); ok {
		out.WriteString(fmt.Sprintf("- **Active Sessions**: %d\n", activeSessions))
	}
	
	// Tool usage
	out.WriteString("\n## Tool Usage Frequency\n\n")
	if toolCounts, ok := summary["tool_usage_counts"].(map[string]int); ok {
		// Convert to slice for sorting
		type toolStat struct {
			name  string
			count int
		}
		var tools []toolStat
		for name, count := range toolCounts {
			tools = append(tools, toolStat{name, count})
		}
		sort.Slice(tools, func(i, j int) bool {
			return tools[i].count > tools[j].count
		})
		
		for _, tool := range tools {
			out.WriteString(fmt.Sprintf("- **%s**: %d uses\n", tool.name, tool.count))
		}
	}
	
	// Success rates
	out.WriteString("\n## Tool Success Rates\n\n")
	if successRates, ok := summary["tool_success_rates"].(map[string]float64); ok {
		for toolName, rate := range successRates {
			out.WriteString(fmt.Sprintf("- **%s**: %.1f%%\n", toolName, rate))
		}
	}
	
	// Search patterns
	out.WriteString("\n## Popular Search Terms\n\n")
	if searchTerms, ok := summary["popular_search_terms"].(map[string]int); ok {
		// Convert to slice for sorting
		type searchStat struct {
			term  string
			count int
		}
		var searches []searchStat
		for term, count := range searchTerms {
			searches = append(searches, searchStat{term, count})
		}
		sort.Slice(searches, func(i, j int) bool {
			return searches[i].count > searches[j].count
		})
		
		// Show top 10
		limit := 10
		if len(searches) < limit {
			limit = len(searches)
		}
		for i := 0; i < limit; i++ {
			out.WriteString(fmt.Sprintf("- **\"%s\"**: %d searches\n", searches[i].term, searches[i].count))
		}
	}
	
	// Session insights
	out.WriteString("\n## Session Insights\n\n")
	if sessionSummary, ok := summary["session_summary"].(map[string]interface{}); ok {
		if avgDuration, ok := sessionSummary["avg_session_duration_minutes"].(float64); ok {
			out.WriteString(fmt.Sprintf("- **Average Session Duration**: %.1f minutes\n", avgDuration))
		}
		if avgTools, ok := sessionSummary["avg_tools_per_session"].(float64); ok {
			out.WriteString(fmt.Sprintf("- **Average Tools per Session**: %.1f\n", avgTools))
		}
		if longestSession, ok := sessionSummary["longest_session_minutes"].(float64); ok {
			out.WriteString(fmt.Sprintf("- **Longest Session**: %.1f minutes\n", longestSession))
		}
		if mostTools, ok := sessionSummary["most_tools_in_session"].(int); ok {
			out.WriteString(fmt.Sprintf("- **Most Tools in Single Session**: %d\n", mostTools))
		}
	}
	
	return mcp.NewToolResultText(out.String()), nil
}

func generateDetailedReport(period string) (*mcp.CallToolResult, error) {
	data := ExportAnalyticsData()
	
	var analyticsData AnalyticsData
	if err := json.Unmarshal([]byte(data), &analyticsData); err != nil {
		return mcp.NewToolResultError("Failed to parse analytics data"), nil
	}
	
	var out strings.Builder
	out.WriteString("# XMLUI-MCP Detailed Analytics Report\n\n")
	
	// Filter by period if needed
	filteredInvocations := filterInvocationsByPeriod(analyticsData.ToolInvocations, period)
	filteredSearches := filterSearchesByPeriod(analyticsData.SearchQueries, period)
	
	out.WriteString(fmt.Sprintf("## Report Period: %s\n", period))
	out.WriteString(fmt.Sprintf("- **Invocations in period**: %d\n", len(filteredInvocations)))
	out.WriteString(fmt.Sprintf("- **Searches in period**: %d\n\n", len(filteredSearches)))
	
	// Tool performance details
	out.WriteString("## Tool Performance Details\n\n")
	toolPerformance := analyzeToolPerformance(filteredInvocations)
	for toolName, perf := range toolPerformance {
		out.WriteString(fmt.Sprintf("### %s\n", toolName))
		out.WriteString(fmt.Sprintf("- Uses: %d\n", perf.Count))
		out.WriteString(fmt.Sprintf("- Success Rate: %.1f%%\n", perf.SuccessRate))
		out.WriteString(fmt.Sprintf("- Avg Duration: %dms\n", perf.AvgDuration.Milliseconds()))
		out.WriteString(fmt.Sprintf("- Avg Result Size: %d chars\n\n", perf.AvgResultSize))
	}
	
	// Search analysis
	out.WriteString("## Search Query Analysis\n\n")
	searchAnalysis := analyzeSearchQueries(filteredSearches)
	out.WriteString(fmt.Sprintf("- **Total Unique Queries**: %d\n", len(searchAnalysis.UniqueQueries)))
	out.WriteString(fmt.Sprintf("- **Average Results per Query**: %.1f\n", searchAnalysis.AvgResultCount))
	out.WriteString(fmt.Sprintf("- **Search Success Rate**: %.1f%%\n\n", searchAnalysis.SuccessRate))
	
	out.WriteString("### Most Common Query Patterns:\n")
	for pattern, count := range searchAnalysis.QueryPatterns {
		out.WriteString(fmt.Sprintf("- %s: %d times\n", pattern, count))
	}
	
	return mcp.NewToolResultText(out.String()), nil
}

// Helper structures for detailed analysis
type ToolPerformance struct {
	Count         int
	SuccessRate   float64
	AvgDuration   time.Duration
	AvgResultSize int
}

type SearchAnalysis struct {
	UniqueQueries    map[string]int
	AvgResultCount   float64
	SuccessRate      float64
	QueryPatterns    map[string]int
}

func filterInvocationsByPeriod(invocations []ToolInvocation, period string) []ToolInvocation {
	if period == "all" {
		return invocations
	}
	
	now := time.Now()
	var cutoff time.Time
	
	switch period {
	case "today":
		cutoff = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		cutoff = now.AddDate(0, 0, -7)
	case "month":
		cutoff = now.AddDate(0, -1, 0)
	default:
		return invocations
	}
	
	var filtered []ToolInvocation
	for _, inv := range invocations {
		if inv.Timestamp.After(cutoff) {
			filtered = append(filtered, inv)
		}
	}
	return filtered
}

func filterSearchesByPeriod(searches []SearchQuery, period string) []SearchQuery {
	if period == "all" {
		return searches
	}
	
	now := time.Now()
	var cutoff time.Time
	
	switch period {
	case "today":
		cutoff = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		cutoff = now.AddDate(0, 0, -7)
	case "month":
		cutoff = now.AddDate(0, -1, 0)
	default:
		return searches
	}
	
	var filtered []SearchQuery
	for _, search := range searches {
		if search.Timestamp.After(cutoff) {
			filtered = append(filtered, search)
		}
	}
	return filtered
}

func analyzeToolPerformance(invocations []ToolInvocation) map[string]ToolPerformance {
	performance := make(map[string]ToolPerformance)
	
	for _, inv := range invocations {
		perf := performance[inv.ToolName]
		perf.Count++
		if inv.Success {
			perf.SuccessRate = (perf.SuccessRate*float64(perf.Count-1) + 100) / float64(perf.Count)
		} else {
			perf.SuccessRate = (perf.SuccessRate * float64(perf.Count-1)) / float64(perf.Count)
		}
		
		// Update average duration
		perf.AvgDuration = (perf.AvgDuration*time.Duration(perf.Count-1) + inv.Duration) / time.Duration(perf.Count)
		
		// Update average result size
		perf.AvgResultSize = (perf.AvgResultSize*(perf.Count-1) + inv.ResultSize) / perf.Count
		
		performance[inv.ToolName] = perf
	}
	
	return performance
}

func analyzeSearchQueries(searches []SearchQuery) SearchAnalysis {
	analysis := SearchAnalysis{
		UniqueQueries: make(map[string]int),
		QueryPatterns: make(map[string]int),
	}
	
	if len(searches) == 0 {
		return analysis
	}
	
	var totalResults int
	var successCount int
	
	for _, search := range searches {
		analysis.UniqueQueries[search.Query]++
		totalResults += search.ResultCount
		
		if search.Success {
			successCount++
		}
		
		// Analyze query patterns (simple keyword extraction)
		words := strings.Fields(strings.ToLower(search.Query))
		for _, word := range words {
			if len(word) > 2 { // Ignore very short words
				analysis.QueryPatterns[word]++
			}
		}
	}
	
	analysis.AvgResultCount = float64(totalResults) / float64(len(searches))
	analysis.SuccessRate = float64(successCount) / float64(len(searches)) * 100
	
	return analysis
}
