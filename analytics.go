package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Duration   time.Duration          `json:"duration_ms"`
	ResultSize int                    `json:"result_size_chars"`
	ErrorMsg   string                 `json:"error_msg,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
}

type SearchQuery struct {
	Type        string        `json:"type"`
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
	Type          string        `json:"type"`
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
	file     *os.File
	writer   *bufio.Writer
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

	// Open file for appending
	a.openFile()

	return a
}

func (a *Analytics) openFile() {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(a.logFile), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create analytics directory: %v\n", err)
		return
	}

	// Open file for appending, create if doesn't exist
	file, err := os.OpenFile(a.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open analytics file: %v\n", err)
		return
	}

	a.file = file
	a.writer = bufio.NewWriter(file)
}

func (a *Analytics) loadData() {
	if _, err := os.Stat(a.logFile); os.IsNotExist(err) {
		return
	}

	file, err := os.Open(a.logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open analytics file for reading: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Try to parse as different types
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse analytics line: %v\n", err)
			continue
		}

		// Determine type and parse accordingly
		if dataType, ok := data["type"].(string); ok {
			switch dataType {
			case "tool_invocation":
				var inv ToolInvocation
				if err := json.Unmarshal([]byte(line), &inv); err == nil {
					a.data.ToolInvocations = append(a.data.ToolInvocations, inv)
				}
			case "search_query":
				var sq SearchQuery
				if err := json.Unmarshal([]byte(line), &sq); err == nil {
					a.data.SearchQueries = append(a.data.SearchQueries, sq)
				}
			case "session_activity":
				var sa SessionActivity
				if err := json.Unmarshal([]byte(line), &sa); err == nil {
					a.data.SessionActivities = append(a.data.SessionActivities, sa)
				}
			}
		}
	}

	// Rebuild sessions map from loaded data
	for _, session := range a.data.SessionActivities {
		sessionCopy := session
		a.sessions[session.SessionID] = &sessionCopy
	}
}

func (a *Analytics) writeLine(data interface{}) {
	if a.writer == nil {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal analytics data: %v\n", err)
		return
	}

	if _, err := a.writer.Write(jsonData); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write analytics data: %v\n", err)
		return
	}

	if _, err := a.writer.Write([]byte("\n")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write newline: %v\n", err)
		return
	}

	// Flush immediately for stdio mode
	if err := a.writer.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to flush analytics data: %v\n", err)
	}
}

func (a *Analytics) LogToolInvocation(toolName string, args map[string]interface{}, success bool, duration time.Duration, resultSize int, errorMsg string, sessionID string) {
	invocation := ToolInvocation{
		Type:       "tool_invocation",
		Timestamp:  time.Now(),
		ToolName:   toolName,
		Arguments:  args,
		Success:    success,
		Duration:   duration,
		ResultSize: resultSize,
		ErrorMsg:   errorMsg,
		SessionID:  sessionID,
	}

	// Write immediately to JSONL file (outside of mutex)
	a.writeLine(invocation)

	// Update in-memory data and session activity (with mutex)
	a.mu.Lock()
	defer a.mu.Unlock()

	// Add to in-memory data
	a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)

	// Update session activity
	a.updateSessionActivity(sessionID, toolName)
}

func (a *Analytics) LogSearchQuery(toolName string, query string, resultCount int, success bool, duration time.Duration, sessionID string, searchPaths []string) {
	searchQuery := SearchQuery{
		Type:        "search_query",
		Timestamp:   time.Now(),
		ToolName:    toolName,
		Query:       query,
		ResultCount: resultCount,
		Success:     success,
		Duration:    duration,
		SessionID:   sessionID,
		SearchPaths: searchPaths,
	}

	// Write immediately to JSONL file (outside of mutex)
	a.writeLine(searchQuery)

	// Add to in-memory data (with mutex)
	a.mu.Lock()
	defer a.mu.Unlock()

	a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)
}

func (a *Analytics) updateSessionActivity(sessionID string, toolName string) {
	if sessionID == "" {
		sessionID = "anonymous"
	}

	session, exists := a.sessions[sessionID]
	if !exists {
		session = &SessionActivity{
			Type:         "session_activity",
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

	// Write updated session activity to JSONL file (outside of mutex)
	go func() {
		a.writeLine(*session)
	}()
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

func (a *Analytics) Close() {
	if a.writer != nil {
		a.writer.Flush()
	}
	if a.file != nil {
		a.file.Close()
	}
}

// Global analytics instance
var globalAnalytics *Analytics

func InitializeAnalytics(logFile string) {
	globalAnalytics = NewAnalytics(logFile)
}

func LogTool(toolName string, args map[string]interface{}, success bool, duration time.Duration, resultSize int, errorMsg string, sessionID string) {
	if globalAnalytics != nil {
		globalAnalytics.LogToolInvocation(toolName, args, success, duration, resultSize, errorMsg, sessionID)
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
		globalAnalytics.Close()
	}
}

// ForceSaveAnalytics forces an immediate save of analytics data
func ForceSaveAnalytics() {
	if globalAnalytics != nil {
		globalAnalytics.Close()
	}
}
