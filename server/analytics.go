package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// Analytics structures for tracking agent usage
type ToolInvocation struct {
	SchemaVersion int                    `json:"schema_version,omitempty"`
	InvocationID  string                 `json:"invocation_id,omitempty"`
	Type          string                 `json:"type"`
	Timestamp     time.Time              `json:"timestamp"`
	ToolName      string                 `json:"tool_name"`
	Arguments     map[string]interface{} `json:"arguments"`
	Success       bool                   `json:"success"`
	ResultSize    int                    `json:"result_size_chars"`
	ErrorMsg      string                 `json:"error_msg,omitempty"`
}

type SearchQuery struct {
	SchemaVersion int       `json:"schema_version,omitempty"`
	InvocationID  string    `json:"invocation_id,omitempty"`
	Type          string    `json:"type"`
	Timestamp     time.Time `json:"timestamp"`
	ToolName      string    `json:"tool_name"`
	Query         string    `json:"query"`

	ExecutionSuccess *bool    `json:"execution_success,omitempty"`
	YieldedResults   *bool    `json:"yielded_results,omitempty"`
	MatchedFileCount *int     `json:"matched_file_count,omitempty"`
	MatchedLineCount *int     `json:"matched_line_count,omitempty"`
	TopScore         *float64 `json:"top_score,omitempty"`
	ScoreGap         *float64 `json:"score_gap,omitempty"`
	TitleMatchCount  *int     `json:"title_match_count,omitempty"`

	SalientTerms             []string `json:"salient_terms,omitempty"`
	SalientTitleMatchCount   *int     `json:"salient_title_match_count,omitempty"`
	SalientContentMatchCount *int     `json:"salient_content_match_count,omitempty"`
	UnansweredTerms          []string `json:"unanswered_terms,omitempty"`
	MatchedPaths     []string `json:"matched_paths"`
	Confidence       string   `json:"confidence"`
	KeptTokens       []string `json:"kept_tokens"`
	RemovedTokens    []string `json:"removed_tokens"`
	ExpandedTokens   []string `json:"expanded_tokens"`
	CorpusVersion    string   `json:"corpus_version"`

	// Deprecated schema-v1 fields. They remain solely so historical JSONL loads.
	ResultCount int      `json:"result_count,omitempty"`
	Success     bool     `json:"success,omitempty"`
	SearchPaths []string `json:"search_paths,omitempty"`
	FoundURLs   []string `json:"found_urls,omitempty"`
}

type AnalyticsData struct {
	ToolInvocations []ToolInvocation `json:"tool_invocations"`
	SearchQueries   []SearchQuery    `json:"search_queries"`
}

type Analytics struct {
	mu      sync.RWMutex
	data    AnalyticsData
	logFile string
}

func InitializeAnalytics(logFile string) {
	// Initialize analytics storage
	globalAnalytics = newAnalytics(logFile)
	WriteDebugLog("[DEBUG] Initializing analytics with log file: %s\n", logFile)
	WriteDebugLog("[DEBUG] Analytics initialized, globalAnalytics is nil: %v\n", globalAnalytics == nil)
}

func (a *Analytics) GetSummary() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Tool usage frequency
	toolCounts := make(map[string]int)
	successRates := make(map[string]float64)
	toolSuccesses := make(map[string]int)
	avgResultSizes := make(map[string]int)
	toolResultSizes := make(map[string][]int)

	for _, inv := range a.data.ToolInvocations {
		toolCounts[inv.ToolName]++
		if inv.Success {
			toolSuccesses[inv.ToolName]++
		}
		toolResultSizes[inv.ToolName] = append(toolResultSizes[inv.ToolName], inv.ResultSize)
	}

	for tool, count := range toolCounts {
		successRates[tool] = float64(toolSuccesses[tool]) / float64(count) * 100

		// Calculate average result size
		sizes := toolResultSizes[tool]
		if len(sizes) > 0 {
			var total int
			for _, s := range sizes {
				total += s
			}
			avgResultSizes[tool] = total / len(sizes)
		}
	}

	// Search query analysis. Schema-v1 records are intentionally excluded from
	// v2 rates because their text-derived success values are ambiguous.
	searchTerms := make(map[string]int)
	v2Count := 0
	legacyCount := 0
	executionSuccessCount := 0
	retrievalHitCount := 0
	zeroResultCount := 0
	for _, sq := range a.data.SearchQueries {
		searchTerms[sq.Query]++
		if sq.SchemaVersion != 2 || sq.ExecutionSuccess == nil || sq.YieldedResults == nil {
			legacyCount++
			continue
		}
		v2Count++
		if *sq.ExecutionSuccess {
			executionSuccessCount++
			if *sq.YieldedResults {
				retrievalHitCount++
			} else {
				zeroResultCount++
			}
		}
	}

	executionSuccessRate := percentage(executionSuccessCount, v2Count)
	retrievalHitRate := percentage(retrievalHitCount, executionSuccessCount)

	return map[string]interface{}{
		"total_tool_invocations":        len(a.data.ToolInvocations),
		"total_search_queries":          len(a.data.SearchQueries),
		"tool_usage_counts":             toolCounts,
		"tool_success_rates":            successRates,
		"tool_avg_result_sizes":         avgResultSizes,
		"search_v2_query_count":         v2Count,
		"legacy_search_query_count":     legacyCount,
		"search_execution_success_rate": executionSuccessRate,
		"search_retrieval_hit_rate":     retrievalHitRate,
		"search_zero_result_count":      zeroResultCount,
		"popular_search_terms":          searchTerms,
	}
}

func percentage(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}

// Wrapper function to add analytics to any tool handler
func WithAnalytics(toolName string, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// DEBUG: Log entry into withAnalytics wrapper
		WriteDebugLog("[DEBUG] withAnalytics ENTRY: tool=%s\n", toolName)

		// DEBUG: Log before calling original handler
		WriteDebugLog("[DEBUG] withAnalytics BEFORE_HANDLER: tool=%s\n", toolName)

		// Call the original handler
		result, err := handler(ctx, req)

		// DEBUG: Log after calling original handler
		WriteDebugLog("[DEBUG] withAnalytics AFTER_HANDLER: tool=%s, err=%v, result_nil=%v\n", toolName, err, result == nil)

		// Calculate metrics
		success := err == nil && result != nil
		resultSize := 0
		errorMsg := ""

		if result != nil {
			// Estimate result size based on content
			for _, content := range result.Content {
				switch c := content.(type) {
				case *mcp.TextContent:
					resultSize += len(c.Text)
				case mcp.TextContent:
					resultSize += len(c.Text)
				}
			}
		}

		if err != nil {
			errorMsg = err.Error()
		} else if result != nil && !result.IsError {
			// Check if result indicates an error
			for _, content := range result.Content {
				if textContent, ok := content.(*mcp.TextContent); ok {
					if len(textContent.Text) > 0 && textContent.Text[0:1] == "❌" {
						success = false
						errorMsg = "Tool returned error result"
						break
					}
				}
			}
		}

		// DEBUG: Log before calling LogTool
		WriteDebugLog("[DEBUG] withAnalytics BEFORE_LOGGING: tool=%s, success=%v, resultSize=%d\n", toolName, success, resultSize)

		// Log the invocation
		logTool(toolName, req.Params.Arguments, success, resultSize, errorMsg)

		// DEBUG: Log after calling LogTool
		WriteDebugLog("[DEBUG] withAnalytics AFTER_LOGGING: tool=%s\n", toolName)

		prependUpdateNotice(result)

		return result, err
	}
}

// Special wrapper for search tools to capture additional search-specific metrics
func WithSearchAnalytics(toolName string, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := searchQueryArgument(req)
		state := &searchAnalyticsState{
			invocationID: newInvocationID(),
			toolName:     toolName,
			query:        query,
		}
		ctx = context.WithValue(ctx, searchAnalyticsContextKey{}, state)

		result, err := handler(ctx, req)
		toolSuccess, resultSize, errorMsg := toolResultMetrics(result, err)
		logToolWithInvocation(state.invocationID, toolName, req.Params.Arguments, toolSuccess, resultSize, errorMsg)

		// Invalid input and failures before mediation still receive a v2 search
		// observation, but they are operational failures rather than misses.
		if !state.observationLogged {
			recordSearchObservation(ctx, toolName, query, toolSuccess, MediatorJSON{}, "")
		}

		prependUpdateNotice(result)

		return result, err
	}
}

type searchAnalyticsContextKey struct{}

type searchAnalyticsState struct {
	invocationID      string
	toolName          string
	query             string
	observationLogged bool
}

func searchQueryArgument(req mcp.CallToolRequest) string {
	if req.Params.Arguments == nil {
		return ""
	}
	query, _ := req.Params.Arguments["query"].(string)
	return strings.TrimSpace(query)
}

func newInvocationID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

func toolResultMetrics(result *mcp.CallToolResult, err error) (bool, int, string) {
	resultSize := 0
	if result != nil {
		for _, content := range result.Content {
			switch c := content.(type) {
			case *mcp.TextContent:
				resultSize += len(c.Text)
			case mcp.TextContent:
				resultSize += len(c.Text)
			}
		}
	}

	switch {
	case err != nil:
		return false, resultSize, err.Error()
	case result == nil:
		return false, resultSize, "Tool returned no result"
	case result.IsError:
		return false, resultSize, "Tool returned error result"
	default:
		return true, resultSize, ""
	}
}

// ExecuteMediatedSearchWithAnalytics keeps presentation and retrieval metrics
// separate: the human string is returned to the caller, while the structured
// mediator summary is the sole source of search-quality analytics.
func ExecuteMediatedSearchWithAnalytics(
	ctx context.Context,
	toolName string,
	homeDir string,
	cfg MediatorConfig,
	query string,
) (string, error) {
	human, summary, err := ExecuteMediatedSearch(homeDir, cfg, query)
	recordSearchObservation(ctx, toolName, query, err == nil, summary, corpusVersionForDir(homeDir))
	return human, err
}

func corpusVersionForDir(homeDir string) string {
	cleaned := filepath.Clean(homeDir)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return ""
	}
	return filepath.Base(cleaned)
}

func recordSearchObservation(
	ctx context.Context,
	toolName string,
	query string,
	executionSuccess bool,
	summary MediatorJSON,
	corpusVersion string,
) {
	invocationID := newInvocationID()
	if state, ok := ctx.Value(searchAnalyticsContextKey{}).(*searchAnalyticsState); ok {
		invocationID = state.invocationID
		state.toolName = toolName
		state.query = query
		state.observationLogged = true
	}
	logSearchV2(invocationID, toolName, query, executionSuccess, summary, corpusVersion)
}

type rankedMatchedPath struct {
	path  string
	score float64
}

// searchSummaryMetrics carries the per-record search-quality signals derived
// from a mediator summary. topScore/scoreGap/titleMatchCount let a downstream
// miner separate content gaps from discoverability gaps without re-running
// the search (#11).
type searchSummaryMetrics struct {
	matchedFileCount int
	matchedLineCount int
	matchedPaths     []string
	topScore         float64
	scoreGap         float64
	titleMatchCount  int
}

func searchMetricsFromSummary(summary MediatorJSON) searchSummaryMetrics {
	pathScores := make(map[string]float64)
	pathTitleMatch := make(map[string]bool)
	metrics := searchSummaryMetrics{}
	for _, facet := range summary.Facets {
		metrics.matchedLineCount += facet.Matches
	}
	for _, items := range summary.Sections {
		for _, item := range items {
			if current, exists := pathScores[item.Path]; !exists || item.Score > current {
				pathScores[item.Path] = item.Score
			}
			if item.TitleMatch {
				pathTitleMatch[item.Path] = true
			}
		}
	}

	ranked := make([]rankedMatchedPath, 0, len(pathScores))
	for path, score := range pathScores {
		ranked = append(ranked, rankedMatchedPath{path: path, score: score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].path < ranked[j].path
		}
		return ranked[i].score > ranked[j].score
	})

	metrics.matchedPaths = make([]string, 0, len(ranked))
	for _, item := range ranked {
		metrics.matchedPaths = append(metrics.matchedPaths, item.path)
	}
	metrics.matchedFileCount = len(ranked)
	metrics.titleMatchCount = len(pathTitleMatch)
	if len(ranked) > 0 {
		metrics.topScore = ranked[0].score
	}
	if len(ranked) > 1 {
		metrics.scoreGap = ranked[0].score - ranked[1].score
	}
	return metrics
}

func newAnalytics(logFile string) *Analytics {
	a := &Analytics{
		data: AnalyticsData{
			ToolInvocations: make([]ToolInvocation, 0),
			SearchQueries:   make([]SearchQuery, 0),
		},
		logFile: logFile,
	}

	// Load existing data if available
	a.loadData()

	return a
}

func (a *Analytics) loadData() {
	if _, err := os.Stat(a.logFile); os.IsNotExist(err) {
		return
	}

	content, err := os.ReadFile(a.logFile)
	if err != nil {
		WriteDebugLog("Failed to load analytics data: %v\n", err)
		return
	}

	// Parse JSONL format - one JSON object per line
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// First, try to determine the type from the JSON
		var typeCheck struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &typeCheck); err != nil {
			WriteDebugLog("Failed to parse type from analytics line: %s\n", line)
			continue
		}

		// Parse based on type
		switch typeCheck.Type {
		case "tool_invocation":
			var invocation ToolInvocation
			if err := json.Unmarshal([]byte(line), &invocation); err == nil {
				a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)
			} else {
				WriteDebugLog("Failed to parse tool_invocation: %s\n", line)
			}
		case "search_query":
			var searchQuery SearchQuery
			if err := json.Unmarshal([]byte(line), &searchQuery); err == nil {
				a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)
			} else {
				WriteDebugLog("Failed to parse search_query: %s\n", line)
			}
		default:
			// Skip session_activity records
			continue
		}
	}
}

func (a *Analytics) writeLine(data interface{}) {
	WriteDebugLog("[DEBUG] writeLine ENTRY: file=%s\n", a.logFile)

	// Marshal the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_MARSHAL: %v\n", err)
		return
	}

	WriteDebugLog("[DEBUG] writeLine MARSHALED: %s\n", string(jsonData))

	// Open file for appending
	file, err := os.OpenFile(a.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_OPEN: %v\n", err)
		return
	}
	defer file.Close()

	WriteDebugLog("[DEBUG] writeLine FILE_OPENED: %s\n", a.logFile)

	// Write the JSON line
	bytesWritten, err := file.Write(jsonData)
	if err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_WRITE: %v\n", err)
		return
	}

	WriteDebugLog("[DEBUG] writeLine WROTE_JSON: %d bytes\n", bytesWritten)

	// Write newline
	newlineBytes, err := file.Write([]byte("\n"))
	if err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_NEWLINE: %v\n", err)
		return
	}

	WriteDebugLog("[DEBUG] writeLine WROTE_NEWLINE: %d bytes\n", newlineBytes)

	// Sync to ensure it's written immediately
	if err := file.Sync(); err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_SYNC: %v\n", err)
		return
	}

	WriteDebugLog("[DEBUG] writeLine SUCCESS: wrote %d total bytes\n", bytesWritten+newlineBytes)
}

func (a *Analytics) logToolInvocation(invocationID string, toolName string, args map[string]interface{}, success bool, resultSize int, errorMsg string) {
	WriteDebugLog("[DEBUG] LogToolInvocation ENTRY: tool=%s\n", toolName)

	a.mu.Lock()
	defer a.mu.Unlock()

	invocation := ToolInvocation{
		SchemaVersion: 2,
		InvocationID:  invocationID,
		Type:          "tool_invocation",
		Timestamp:     time.Now(),
		ToolName:      toolName,
		Arguments:     args,
		Success:       success,
		ResultSize:    resultSize,
		ErrorMsg:      errorMsg,
	}

	WriteDebugLog("[DEBUG] LogToolInvocation BEFORE_APPEND: tool=%s, current_count=%d\n", toolName, len(a.data.ToolInvocations))

	a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)

	WriteDebugLog("[DEBUG] LogToolInvocation AFTER_APPEND: tool=%s, new_count=%d\n", toolName, len(a.data.ToolInvocations))

	// Write debug info to server.log file
	WriteDebugLog("[DEBUG] LogToolInvocation: tool=%s, success=%v, resultSize=%d\n",
		toolName, success, resultSize)

	// Save immediately for stdio mode (each call is a separate process)
	WriteDebugLog("[DEBUG] LogToolInvocation BEFORE_WRITELINE: calling writeLine\n")
	a.writeLine(invocation)
	WriteDebugLog("[DEBUG] LogToolInvocation AFTER_WRITELINE: writeLine completed\n")
}

func (a *Analytics) logSearchQueryV2(invocationID string, toolName string, query string, executionSuccess bool, summary MediatorJSON, corpusVersion string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	WriteDebugLog("[DEBUG] LogSearchQuery ENTRY: tool=%s, query=%s\n", toolName, query)

	metrics := searchMetricsFromSummary(summary)
	yieldedResults := metrics.matchedFileCount > 0
	executionSuccessValue := executionSuccess
	yieldedResultsValue := yieldedResults
	matchedFileCountValue := metrics.matchedFileCount
	matchedLineCountValue := metrics.matchedLineCount
	titleMatchCountValue := metrics.titleMatchCount
	searchQuery := SearchQuery{
		SchemaVersion:    2,
		InvocationID:     invocationID,
		Type:             "search_query",
		Timestamp:        time.Now(),
		ToolName:         toolName,
		Query:            query,
		ExecutionSuccess: &executionSuccessValue,
		YieldedResults:   &yieldedResultsValue,
		MatchedFileCount: &matchedFileCountValue,
		MatchedLineCount: &matchedLineCountValue,
		TitleMatchCount:  &titleMatchCountValue,
		MatchedPaths:     metrics.matchedPaths,
		Confidence:       summary.Confidence,
		KeptTokens:       append([]string{}, summary.Tokens["kept"]...),
		RemovedTokens:    append([]string{}, summary.Tokens["removed"]...),
		ExpandedTokens:   append([]string{}, summary.Tokens["expanded"]...),
		CorpusVersion:    corpusVersion,
	}
	if yieldedResults {
		topScoreValue := metrics.topScore
		scoreGapValue := metrics.scoreGap
		searchQuery.TopScore = &topScoreValue
		searchQuery.ScoreGap = &scoreGapValue
	}
	if summary.Salience != nil {
		salientTitleValue := summary.Salience.TitleMatchCount
		salientContentValue := summary.Salience.ContentMatchCount
		searchQuery.SalientTerms = append([]string{}, summary.Salience.Terms...)
		searchQuery.SalientTitleMatchCount = &salientTitleValue
		searchQuery.SalientContentMatchCount = &salientContentValue
		searchQuery.UnansweredTerms = append([]string{}, summary.Salience.UnansweredTerms...)
	}

	a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)

	// Debug before writeLine
	WriteDebugLog("[DEBUG] LogSearchQuery BEFORE_WRITELINE: calling writeLine\n")

	// Save immediately for stdio mode
	a.writeLine(searchQuery)

	// Debug after writeLine
	WriteDebugLog("[DEBUG] LogSearchQuery AFTER_WRITELINE: writeLine completed\n")
}

// Global analytics instance
var globalAnalytics *Analytics

var (
	globalUpdateNotice   string
	globalUpdateNoticeMu sync.Mutex
)

// SetUpdateNotice sets a notice that will be prepended to the next successful tool response.
func SetUpdateNotice(notice string) {
	globalUpdateNoticeMu.Lock()
	defer globalUpdateNoticeMu.Unlock()
	globalUpdateNotice = notice
}

func prependUpdateNotice(result *mcp.CallToolResult) {
	if result == nil || result.IsError {
		return
	}

	globalUpdateNoticeMu.Lock()
	defer globalUpdateNoticeMu.Unlock()
	if globalUpdateNotice == "" {
		return
	}

	for i, content := range result.Content {
		switch tc := content.(type) {
		case *mcp.TextContent:
			result.Content[i] = mcp.NewTextContent(globalUpdateNotice + "\n\n" + tc.Text)
			globalUpdateNotice = ""
			return
		case mcp.TextContent:
			result.Content[i] = mcp.NewTextContent(globalUpdateNotice + "\n\n" + tc.Text)
			globalUpdateNotice = ""
			return
		}
	}
}

func logTool(toolName string, args map[string]interface{}, success bool, resultSize int, errorMsg string) {
	logToolWithInvocation("", toolName, args, success, resultSize, errorMsg)
}

func logToolWithInvocation(invocationID string, toolName string, args map[string]interface{}, success bool, resultSize int, errorMsg string) {
	WriteDebugLog("[DEBUG] LogTool ENTRY: tool=%s, globalAnalytics_nil=%v\n", toolName, globalAnalytics == nil)

	if globalAnalytics != nil {
		WriteDebugLog("[DEBUG] LogTool CALLING_LogToolInvocation: tool=%s\n", toolName)
		globalAnalytics.logToolInvocation(invocationID, toolName, args, success, resultSize, errorMsg)
		WriteDebugLog("[DEBUG] LogTool AFTER_LogToolInvocation: tool=%s\n", toolName)
	} else {
		WriteDebugLog("[DEBUG] LogTool SKIPPED: globalAnalytics is nil for tool=%s\n", toolName)
	}
}

func logSearchV2(invocationID string, toolName string, query string, executionSuccess bool, summary MediatorJSON, corpusVersion string) {
	if globalAnalytics != nil {
		WriteDebugLog("[DEBUG] LogSearch ENTRY: tool=%s, query=%s\n", toolName, query)
		globalAnalytics.logSearchQueryV2(invocationID, toolName, query, executionSuccess, summary, corpusVersion)
		WriteDebugLog("[DEBUG] LogSearch AFTER_LogSearchQuery: tool=%s\n", toolName)
	}
}

func GetAnalyticsSummary() map[string]interface{} {
	if globalAnalytics != nil {
		return globalAnalytics.GetSummary()
	}
	return map[string]interface{}{}
}
