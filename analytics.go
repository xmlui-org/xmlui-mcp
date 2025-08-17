package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Analytics structures for tracking agent usage
type ToolInvocation struct {
	Timestamp  time.Time              `json:"timestamp"`
	ToolName   string                 `json:"tool_name"`
	Arguments  map[string]interface{} `json:"arguments"`
	Success    bool                   `json:"success"`
	Duration   time.Duration          `json:"duration_ms"`
	ResultSize int                    `json:"result_size_chars"`
	ErrorMsg   string                 `json:"error_msg,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
}

type SearchQuery struct {
	Timestamp   time.Time     `json:"timestamp"`
	ToolName    string        `json:"tool_name"`
	Query       string        `json:"query"`
	ResultCount int           `json:"result_count"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration_ms"`
	SessionID   string        `json:"session_id,omitempty"`
	SearchPaths []string      `json:"search_paths,omitempty"`
}

type SessionActivity struct {
	SessionID     string        `json:"session_id"`
	StartTime     time.Time     `json:"start_time"`
	LastActivity  time.Time     `json:"last_activity"`
	ToolCount     int           `json:"tool_count"`
	UniqueTools   []string      `json:"unique_tools"`
	TotalDuration time.Duration `json:"total_duration_ms"`
}

type AnalyticsData struct {
	ToolInvocations   []ToolInvocation  `json:"tool_invocations"`
	SearchQueries     []SearchQuery     `json:"search_queries"`
	SessionActivities []SessionActivity `json:"session_activities"`
}

type Analytics struct {
	mu       sync.RWMutex
	data     AnalyticsData
	logFile  string
	sessions map[string]*SessionActivity
}

func NewAnalytics(logFile string) *Analytics {
	a := &Analytics{
		data: AnalyticsData{
			ToolInvocations:   make([]ToolInvocation, 0),
			SearchQueries:     make([]SearchQuery, 0),
			SessionActivities: make([]SessionActivity, 0),
		},
		logFile:  logFile,
		sessions: make(map[string]*SessionActivity),
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

	if err := json.Unmarshal(content, &a.data); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse analytics data: %v\n", err)
		return
	}

	// Rebuild sessions map from loaded data
	for _, session := range a.data.SessionActivities {
		sessionCopy := session
		a.sessions[session.SessionID] = &sessionCopy
	}
}

func (a *Analytics) writeLine(data interface{}) {
	// Marshal the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal analytics data: %v\n", err)
		return
	}

	// Open file for appending
	file, err := os.OpenFile(a.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open analytics file: %v\n", err)
		return
	}
	defer file.Close()

	// Write the JSON line
	if _, err := file.Write(jsonData); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write analytics data: %v\n", err)
		return
	}

	// Write newline
	if _, err := file.Write([]byte("\n")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write newline: %v\n", err)
		return
	}

	// Sync to ensure it's written immediately
	file.Sync()
}

func (a *Analytics) saveData() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Update session activities from sessions map
	a.data.SessionActivities = make([]SessionActivity, 0, len(a.sessions))
	for _, session := range a.sessions {
		a.data.SessionActivities = append(a.data.SessionActivities, *session)
	}

	data, err := json.MarshalIndent(a.data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal analytics data: %v\n", err)
		return
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(a.logFile), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create analytics directory: %v\n", err)
		return
	}

	if err := os.WriteFile(a.logFile, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save analytics data: %v\n", err)
	}
}

func (a *Analytics) LogToolInvocation(toolName string, args map[string]interface{}, success bool, duration time.Duration, resultSize int, errorMsg string, sessionID string) {
	fmt.Fprintf(os.Stderr, "[DEBUG] LogToolInvocation ENTRY: tool=%s, session=%s\n", toolName, sessionID)

	a.mu.Lock()
	defer a.mu.Unlock()

	invocation := ToolInvocation{
		Timestamp:  time.Now(),
		ToolName:   toolName,
		Arguments:  args,
		Success:    success,
		Duration:   duration,
		ResultSize: resultSize,
		ErrorMsg:   errorMsg,
		SessionID:  sessionID,
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] LogToolInvocation BEFORE_APPEND: tool=%s, session=%s, current_count=%d\n", toolName, sessionID, len(a.data.ToolInvocations))

	a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)

	fmt.Fprintf(os.Stderr, "[DEBUG] LogToolInvocation AFTER_APPEND: tool=%s, session=%s, new_count=%d\n", toolName, sessionID, len(a.data.ToolInvocations))

	// Update session activity
	a.updateSessionActivity(sessionID, toolName)

	// Write debug info to server.log file
	writeDebugLog("[DEBUG] LogToolInvocation: tool=%s, session=%s, success=%v, duration=%v, resultSize=%d\n",
		toolName, sessionID, success, duration, resultSize)

	// Save immediately for stdio mode (each call is a separate process)
	a.saveData()
}

func (a *Analytics) LogSearchQuery(toolName string, query string, resultCount int, success bool, duration time.Duration, sessionID string, searchPaths []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	searchQuery := SearchQuery{
		Timestamp:   time.Now(),
		ToolName:    toolName,
		Query:       query,
		ResultCount: resultCount,
		Success:     success,
		Duration:    duration,
		SessionID:   sessionID,
		SearchPaths: searchPaths,
	}

	a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)

	// Save immediately for stdio mode
	a.saveData()
}

func (a *Analytics) updateSessionActivity(sessionID string, toolName string) {
	if sessionID == "" {
		sessionID = "anonymous"
	}

	session, exists := a.sessions[sessionID]
	if !exists {
		session = &SessionActivity{
			SessionID:    sessionID,
			StartTime:    time.Now(),
			LastActivity: time.Now(),
			ToolCount:    0,
			UniqueTools:  make([]string, 0),
		}
		a.sessions[sessionID] = session
	}

	session.LastActivity = time.Now()
	session.ToolCount++
	session.TotalDuration = session.LastActivity.Sub(session.StartTime)

	// Add to unique tools if not already present
	found := false
	for _, tool := range session.UniqueTools {
		if tool == toolName {
			found = true
			break
		}
	}
	if !found {
		session.UniqueTools = append(session.UniqueTools, toolName)
	}
}

func (a *Analytics) GetSummary() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Tool usage frequency
	toolCounts := make(map[string]int)
	successRates := make(map[string]float64)
	toolSuccesses := make(map[string]int)
	avgDurations := make(map[string]time.Duration)
	toolDurations := make(map[string][]time.Duration)

	for _, inv := range a.data.ToolInvocations {
		toolCounts[inv.ToolName]++
		if inv.Success {
			toolSuccesses[inv.ToolName]++
		}
		toolDurations[inv.ToolName] = append(toolDurations[inv.ToolName], inv.Duration)
	}

	for tool, count := range toolCounts {
		successRates[tool] = float64(toolSuccesses[tool]) / float64(count) * 100

		// Calculate average duration
		durations := toolDurations[tool]
		if len(durations) > 0 {
			var total time.Duration
			for _, d := range durations {
				total += d
			}
			avgDurations[tool] = total / time.Duration(len(durations))
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
		"active_sessions":        len(a.sessions),
		"tool_usage_counts":      toolCounts,
		"tool_success_rates":     successRates,
		"tool_avg_durations_ms":  avgDurations,
		"search_success_rate":    searchSuccessRate,
		"popular_search_terms":   searchTerms,
		"session_summary":        a.getSessionSummary(),
	}
}

func (a *Analytics) getSessionSummary() map[string]interface{} {
	if len(a.sessions) == 0 {
		return map[string]interface{}{}
	}

	var totalDuration time.Duration
	var totalTools int
	longestSession := time.Duration(0)
	mostActiveSession := 0

	for _, session := range a.sessions {
		totalDuration += session.TotalDuration
		totalTools += session.ToolCount

		if session.TotalDuration > longestSession {
			longestSession = session.TotalDuration
		}

		if session.ToolCount > mostActiveSession {
			mostActiveSession = session.ToolCount
		}
	}

	return map[string]interface{}{
		"avg_session_duration_minutes": totalDuration.Minutes() / float64(len(a.sessions)),
		"avg_tools_per_session":        float64(totalTools) / float64(len(a.sessions)),
		"longest_session_minutes":      longestSession.Minutes(),
		"most_tools_in_session":        mostActiveSession,
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

func InitializeAnalytics(logFile string) {
	writeDebugLog("[DEBUG] Initializing analytics with log file: %s\n", logFile)
	globalAnalytics = NewAnalytics(logFile)
	writeDebugLog("[DEBUG] Analytics initialized, globalAnalytics is nil: %v\n", globalAnalytics == nil)
}

func LogTool(toolName string, args map[string]interface{}, success bool, duration time.Duration, resultSize int, errorMsg string, sessionID string) {
	writeDebugLog("[DEBUG] LogTool ENTRY: tool=%s, session=%s, globalAnalytics_nil=%v\n", toolName, sessionID, globalAnalytics == nil)

	if globalAnalytics != nil {
		writeDebugLog("[DEBUG] LogTool CALLING_LogToolInvocation: tool=%s, session=%s\n", toolName, sessionID)
		globalAnalytics.LogToolInvocation(toolName, args, success, duration, resultSize, errorMsg, sessionID)
		writeDebugLog("[DEBUG] LogTool AFTER_LogToolInvocation: tool=%s, session=%s\n", toolName, sessionID)
	} else {
		writeDebugLog("[DEBUG] LogTool SKIPPED: globalAnalytics is nil for tool=%s, session=%s\n", toolName, sessionID)
	}
}

func LogSearch(toolName string, query string, resultCount int, success bool, duration time.Duration, sessionID string, searchPaths []string) {
	if globalAnalytics != nil {
		globalAnalytics.LogSearchQuery(toolName, query, resultCount, success, duration, sessionID, searchPaths)
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

func SaveAnalytics() {
	if globalAnalytics != nil {
		globalAnalytics.saveData()
	}
}

// ForceSaveAnalytics forces an immediate save of analytics data
func ForceSaveAnalytics() {
	if globalAnalytics != nil {
		globalAnalytics.saveData()
	}
}
