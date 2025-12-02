package mcpsvr

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mikeschinkel/go-dt"
	"github.com/mikeschinkel/go-logutil"
)

type LogEntryType string

const (
	ToolInvocationType LogEntryType = "tool_invocation"
	SearchQueryType    LogEntryType = "search_query"
)

// Analytics structures for tracking agent usage

type ToolInvocation struct {
	EntryType  LogEntryType           `json:"entry_type"`
	Timestamp  time.Time              `json:"timestamp"`
	ToolName   string                 `json:"tool_name"`
	Arguments  map[string]interface{} `json:"arguments"`
	Success    bool                   `json:"success"`
	ResultSize int                    `json:"result_size_chars"`
	ErrorMsg   string                 `json:"error_msg,omitempty"`
}

type SearchQuery struct {
	EntryType   LogEntryType `json:"entry_type"`
	Timestamp   time.Time    `json:"timestamp"`
	ToolName    string       `json:"tool_name"`
	Query       string       `json:"query"`
	ResultCount int          `json:"result_count"`
	Success     bool         `json:"success"`
	SearchPaths []string     `json:"search_paths,omitempty"`
	FoundURLs   []string     `json:"found_urls,omitempty"`
}

type AnalyticsData struct {
	ToolInvocations []ToolInvocation `json:"tool_invocations"`
	SearchQueries   []SearchQuery    `json:"search_queries"`
}

type Analytics struct {
	mu     sync.RWMutex
	data   AnalyticsData
	logger *slog.Logger
}

func NewAnalytics(logger *slog.Logger) *Analytics {
	return &Analytics{
		data: AnalyticsData{
			ToolInvocations: make([]ToolInvocation, 0),
			SearchQueries:   make([]SearchQuery, 0),
		},
		logger: logger,
	}
}

func (a *Analytics) Initialize() (err error) {

	// Load existing data if available
	err = a.loadData()

	return err
}

func (a *Analytics) loadData() (err error) {
	const maxLineSize = 1024 * 1024
	var workFile *os.File
	var logFilename, workFilename dt.Filepath
	var scanner *bufio.Scanner
	var buf []byte

	workFile, err = dt.CreateTemp(dt.TempDir(), "xmlui-analytics-*.json")
	if err != nil {
		a.logger.Debug("Failed to create temporary logFile")
		goto end
	}
	defer dt.CloseOrLog(workFile)

	logFilename = logutil.GetJSONFilepath(a.logger)
	workFilename = dt.Filepath(workFile.Name())
	defer dt.LogOnError(workFilename.Remove())
	err = logFilename.CopyTo(workFilename, &dt.CopyOptions{
		Overwrite: true,
	})
	if !os.IsNotExist(err) && err != nil {
		a.logger.Debug("Failed to copy log logFile",
			"log_file", logFilename,
			"working_file", workFilename,
			"err", err,
		)
		goto end
	}
	_, err = workFile.Seek(0, io.SeekStart)
	if err != nil {
		a.logger.Debug("Failed to reset work file back to start of file",
			"data_file", workFilename,
			"err", err,
		)
		goto end
	}
	scanner = bufio.NewScanner(workFile)
	buf = make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLineSize)

	// Parse JSONL format - one JSON object per line
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Do whatever you need with each line:
		a.parseJSONLine(line)
	}

	err = scanner.Err()

end:
	if err != nil {
		a.logger.Debug("Failed while loading analytics data", "error", err)
	}
	return
}

func (a *Analytics) parseJSONLine(line string) {

	// First, try to determine the type from the JSON
	var typeCheck struct {
		EntryType LogEntryType `json:"entry_type"`
	}

	err := json.Unmarshal([]byte(line), &typeCheck)
	if err != nil {
		a.logger.Debug("Failed to parse entry type in analytics", "line", line)
		goto end
	}

	// Parse based on type
	switch typeCheck.EntryType {
	case ToolInvocationType:
		var invocation ToolInvocation
		err := json.Unmarshal([]byte(line), &invocation)
		if err != nil {
			a.logger.Debug("Failed to parse tool_invocation", "line", line, "error", err)
			goto end
		}
		a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)

	case SearchQueryType:
		var searchQuery SearchQuery
		err := json.Unmarshal([]byte(line), &searchQuery)
		if err != nil {
			a.logger.Debug("Failed to parse search_query", "line", line, "error", err)
			goto end
		}
		a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)
	default:
		// Skip session_activity records
	}
end:
	return
}

func (a *Analytics) LogToolInvocation(toolName string, args map[string]interface{}, success bool, resultSize int, errorMsg string) {
	a.logger.Debug("LogToolInvocation ENTRY",
		slog.String("tool", toolName),
	)

	a.mu.Lock()
	defer a.mu.Unlock()

	invocation := ToolInvocation{
		EntryType:  "tool_invocation",
		Timestamp:  time.Now(),
		ToolName:   toolName,
		Arguments:  args,
		Success:    success,
		ResultSize: resultSize,
		ErrorMsg:   errorMsg,
	}

	a.logger.Debug("LogToolInvocation BEFORE_APPEND: tool=%s, current_count=%d\n", toolName, len(a.data.ToolInvocations))

	a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)

	a.logger.Debug("LogToolInvocation AFTER_APPEND",
		slog.String("tool", toolName),
		slog.Bool("success", success),
		slog.Int("new_count", len(a.data.ToolInvocations)),
	)

	// Write debug info to server.log file
	a.logger.Debug("LogToolInvocation",
		slog.String("tool", toolName),
		slog.Bool("success", success),
		slog.Int("resultSize", resultSize),
	)

	// Save immediately for stdio mode (each call is a separate process)
	a.logger.Debug("LogToolInvocation BEFORE_WRITELINE")
	a.logger.Debug("LogToolInvocation", logutil.LogArgs(invocation)...)
	a.logger.Debug("LogToolInvocation AFTER_WRITELINE")
}

func (a *Analytics) LogSearchQuery(toolName string, query string, resultCount int, success bool, searchPaths []string, foundURLs []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Debug to server.log instead of analytics file
	a.logger.Debug("LogSearchQuery ENTRY",
		slog.String("tool", toolName),
		slog.String("query", query),
	)

	searchQuery := SearchQuery{
		EntryType:   ToolInvocationType,
		Timestamp:   time.Now(),
		ToolName:    toolName,
		Query:       query,
		ResultCount: resultCount,
		Success:     success,
		SearchPaths: searchPaths,
		FoundURLs:   foundURLs,
	}

	a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)

	// Debug before writeLine
	a.logger.Debug("LogSearchQuery BEFORE_WRITELINE")
	// Save immediately for stdio mode
	a.logger.Debug("LogToolInvocation", logutil.LogArgs(searchQuery)...)
	// Debug after writeLine
	a.logger.Debug("LogSearchQuery AFTER_WRITELINE")
}

func (a *Analytics) GetSummary() map[string]any {
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

	// Search query analysis
	searchTerms := make(map[string]int)
	searchSuccessRate := 0.0
	if len(a.data.SearchQueries) > 0 {
		successCount := 0
		for _, sq := range a.data.SearchQueries {
			searchTerms[sq.Query]++
			if sq.Success {
				successCount++
			}
		}
		searchSuccessRate = float64(successCount) / float64(len(a.data.SearchQueries)) * 100
	}

	return map[string]interface{}{
		"total_tool_invocations": len(a.data.ToolInvocations),
		"total_search_queries":   len(a.data.SearchQueries),
		"tool_usage_counts":      toolCounts,
		"tool_success_rates":     successRates,
		"tool_avg_result_sizes":  avgResultSizes,
		"search_success_rate":    searchSuccessRate,
		"popular_search_terms":   searchTerms,
	}
}

// Global analytics instance
var globalAnalytics *Analytics

func InitializeAnalytics(logger *slog.Logger) (err error) {
	var logFile dt.Filepath
	var attr slog.Attr

	// Initialize analytics storage
	globalAnalytics = NewAnalytics(logger)
	err = globalAnalytics.Initialize()
	if err != nil {
		logger.Debug("globalAnalytics failed to initialize", "error", err)
		goto end
	}
	// Ensure server.log is written next to analytics file
	logFile = logutil.GetJSONFilepath(logger)
	if logFile != "" {
		attr = slog.String("log_file", string(logFile))
	}
	logger.Debug("Initializing analytics", attr)
end:
	return err
}

func LogTool(toolName string, args map[string]interface{}, success bool, resultSize int, errorMsg string, logger *slog.Logger) {
	toolLogger := logger.With("tool", toolName)
	toolLogger.Debug("LogTool ENTRY",
		slog.Bool("globalAnalytics_nil=%", globalAnalytics == nil),
	)

	if globalAnalytics == nil {
		toolLogger.Debug("LogTool SKIPPED: globalAnalytics is nil")
		goto end
	}
	toolLogger.Debug("LogTool CALLING_LogToolInvocation")
	globalAnalytics.LogToolInvocation(toolName, args, success, resultSize, errorMsg)
	toolLogger.Debug("LogTool AFTER_LogToolInvocation")
end:
	return
}

func LogSearch(toolName string, query string, resultCount int, success bool, searchPaths []string, foundURLs []string, logger *slog.Logger) {
	toolLogger := logger.With("tool", toolName)
	if globalAnalytics == nil {
		goto end
	}
	toolLogger.Debug("LogSearch ENTRY", slog.String("query", query))
	globalAnalytics.LogSearchQuery(toolName, query, resultCount, success, searchPaths, foundURLs)
	toolLogger.Debug("LogSearch AFTER_LogSearchQuery")
end:
	return
}

func GetAnalyticsSummary() map[string]interface{} {
	if globalAnalytics != nil {
		return globalAnalytics.GetSummary()
	}
	return map[string]interface{}{}
}
