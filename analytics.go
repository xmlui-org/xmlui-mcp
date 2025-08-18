package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Analytics structures for tracking agent usage
type ToolInvocation struct {
	Type       string                 `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	ToolName   string                 `json:"tool_name"`
	Arguments  map[string]interface{} `json:"arguments"`
	Success    bool                   `json:"success"`
	ResultSize int                    `json:"result_size_chars"`
	ErrorMsg   string                 `json:"error_msg,omitempty"`
}

type SearchQuery struct {
	Type        string    `json:"type"`
	Timestamp   time.Time `json:"timestamp"`
	ToolName    string    `json:"tool_name"`
	Query       string    `json:"query"`
	ResultCount int       `json:"result_count"`
	Success     bool      `json:"success"`
	SearchPaths []string  `json:"search_paths,omitempty"`
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

func NewAnalytics(logFile string) *Analytics {
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
		fmt.Fprintf(os.Stderr, "Failed to load analytics data: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "Failed to parse type from analytics line: %s\n", line)
			continue
		}

		// Parse based on type
		switch typeCheck.Type {
		case "tool_invocation":
			var invocation ToolInvocation
			if err := json.Unmarshal([]byte(line), &invocation); err == nil {
				a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)
			} else {
				fmt.Fprintf(os.Stderr, "Failed to parse tool_invocation: %s\n", line)
			}
		case "search_query":
			var searchQuery SearchQuery
			if err := json.Unmarshal([]byte(line), &searchQuery); err == nil {
				a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)
			} else {
				fmt.Fprintf(os.Stderr, "Failed to parse search_query: %s\n", line)
			}
		default:
			// Skip session_activity records
			continue
		}
	}
}

func (a *Analytics) writeLine(data interface{}) {
	writeDebugLog("[DEBUG] writeLine ENTRY: file=%s\n", a.logFile)

	// Marshal the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		writeDebugLog("[DEBUG] writeLine FAILED_MARSHAL: %v\n", err)
		return
	}

	writeDebugLog("[DEBUG] writeLine MARSHALED: %s\n", string(jsonData))

	// Open file for appending
	file, err := os.OpenFile(a.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		writeDebugLog("[DEBUG] writeLine FAILED_OPEN: %v\n", err)
		return
	}
	defer file.Close()

	writeDebugLog("[DEBUG] writeLine FILE_OPENED: %s\n", a.logFile)

	// Write the JSON line
	bytesWritten, err := file.Write(jsonData)
	if err != nil {
		writeDebugLog("[DEBUG] writeLine FAILED_WRITE: %v\n", err)
		return
	}

	writeDebugLog("[DEBUG] writeLine WROTE_JSON: %d bytes\n", bytesWritten)

	// Write newline
	newlineBytes, err := file.Write([]byte("\n"))
	if err != nil {
		writeDebugLog("[DEBUG] writeLine FAILED_NEWLINE: %v\n", err)
		return
	}

	writeDebugLog("[DEBUG] writeLine WROTE_NEWLINE: %d bytes\n", newlineBytes)

	// Sync to ensure it's written immediately
	if err := file.Sync(); err != nil {
		writeDebugLog("[DEBUG] writeLine FAILED_SYNC: %v\n", err)
		return
	}

	writeDebugLog("[DEBUG] writeLine SUCCESS: wrote %d total bytes\n", bytesWritten+newlineBytes)
}

func (a *Analytics) LogToolInvocation(toolName string, args map[string]interface{}, success bool, resultSize int, errorMsg string) {
	writeDebugLog("[DEBUG] LogToolInvocation ENTRY: tool=%s\n", toolName)

	a.mu.Lock()
	defer a.mu.Unlock()

	invocation := ToolInvocation{
		Type:       "tool_invocation",
		Timestamp:  time.Now(),
		ToolName:   toolName,
		Arguments:  args,
		Success:    success,
		ResultSize: resultSize,
		ErrorMsg:   errorMsg,
	}

	writeDebugLog("[DEBUG] LogToolInvocation BEFORE_APPEND: tool=%s, current_count=%d\n", toolName, len(a.data.ToolInvocations))

	a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)

	writeDebugLog("[DEBUG] LogToolInvocation AFTER_APPEND: tool=%s, new_count=%d\n", toolName, len(a.data.ToolInvocations))

	// Write debug info to server.log file
	writeDebugLog("[DEBUG] LogToolInvocation: tool=%s, success=%v, resultSize=%d\n",
		toolName, success, resultSize)

	// Save immediately for stdio mode (each call is a separate process)
	writeDebugLog("[DEBUG] LogToolInvocation BEFORE_WRITELINE: calling writeLine\n")
	a.writeLine(invocation)
	writeDebugLog("[DEBUG] LogToolInvocation AFTER_WRITELINE: writeLine completed\n")
}

func (a *Analytics) LogSearchQuery(toolName string, query string, resultCount int, success bool, searchPaths []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Debug to server.log instead of analytics file
	writeDebugLog("[DEBUG] LogSearchQuery ENTRY: tool=%s, query=%s\n", toolName, query)

	searchQuery := SearchQuery{
		Type:        "search_query",
		Timestamp:   time.Now(),
		ToolName:    toolName,
		Query:       query,
		ResultCount: resultCount,
		Success:     success,
		SearchPaths: searchPaths,
	}

	a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)

	// Debug before writeLine
	writeDebugLog("[DEBUG] LogSearchQuery BEFORE_WRITELINE: calling writeLine\n")

	// Save immediately for stdio mode
	a.writeLine(searchQuery)

	// Debug after writeLine
	writeDebugLog("[DEBUG] LogSearchQuery AFTER_WRITELINE: writeLine completed\n")
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

func (a *Analytics) ExportData() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	data, err := json.MarshalIndent(a.data, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error exporting data: %v", err)
	}
	return string(data)
}

// Global analytics instance
var globalAnalytics *Analytics

// Global debug log path for server.log; set alongside analytics file
var debugLogPath string

func InitializeAnalytics(logFile string) {
	// Initialize analytics storage
	globalAnalytics = NewAnalytics(logFile)
	// Ensure server.log is written next to analytics file
	debugLogPath = filepath.Join(filepath.Dir(logFile), "server.log")
	writeDebugLog("[DEBUG] Initializing analytics with log file: %s\n", logFile)
	writeDebugLog("[DEBUG] Analytics initialized, globalAnalytics is nil: %v\n", globalAnalytics == nil)
}

func LogTool(toolName string, args map[string]interface{}, success bool, resultSize int, errorMsg string) {
	writeDebugLog("[DEBUG] LogTool ENTRY: tool=%s, globalAnalytics_nil=%v\n", toolName, globalAnalytics == nil)

	if globalAnalytics != nil {
		writeDebugLog("[DEBUG] LogTool CALLING_LogToolInvocation: tool=%s\n", toolName)
		globalAnalytics.LogToolInvocation(toolName, args, success, resultSize, errorMsg)
		writeDebugLog("[DEBUG] LogTool AFTER_LogToolInvocation: tool=%s\n", toolName)
	} else {
		writeDebugLog("[DEBUG] LogTool SKIPPED: globalAnalytics is nil for tool=%s\n", toolName)
	}
}

func LogSearch(toolName string, query string, resultCount int, success bool, searchPaths []string) {
	if globalAnalytics != nil {
		writeDebugLog("[DEBUG] LogSearch ENTRY: tool=%s, query=%s\n", toolName, query)
		globalAnalytics.LogSearchQuery(toolName, query, resultCount, success, searchPaths)
		writeDebugLog("[DEBUG] LogSearch AFTER_LogSearchQuery: tool=%s\n", toolName)
	}
}

func GetAnalyticsSummary() map[string]interface{} {
	if globalAnalytics != nil {
		return globalAnalytics.GetSummary()
	}
	return map[string]interface{}{}
}

func ExportAnalyticsData() string {
	if globalAnalytics != nil {
		return globalAnalytics.ExportData()
	}
	return "{}"
}

// SaveAnalytics is no longer needed with JSONL format - each entry is written immediately
func SaveAnalytics() {
	// No-op: JSONL format writes each entry immediately
}

// ForceSaveAnalytics is no longer needed with JSONL format - each entry is written immediately
func ForceSaveAnalytics() {
	// No-op: JSONL format writes each entry immediately
}
