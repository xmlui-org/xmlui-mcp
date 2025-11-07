package xmluimcp

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// GetOrCreateSession gets an existing session or creates a new one
func (sm *SessionManager) GetOrCreateSession(sessionID string) *SessionContext {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		session.LastActivity = time.Now()
		return session
	}

	// Create new session
	session := &SessionContext{
		ID:               sessionID,
		InjectedPrompts:  []string{},
		LastActivity:     time.Now(),
		Context:          []mcp.PromptMessage{},
	}
	sm.sessions[sessionID] = session
	return session
}

// InjectPrompt injects a prompt into a session context
func (sm *SessionManager) InjectPrompt(sessionID string, promptName string, promptHandlers map[string]PromptHandler) (*InjectPromptResponse, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Get or create session
	session, exists := sm.sessions[sessionID]
	if !exists {
		session = &SessionContext{
			ID:               sessionID,
			InjectedPrompts:  []string{},
			LastActivity:     time.Now(),
			Context:          []mcp.PromptMessage{},
		}
		sm.sessions[sessionID] = session
	}

	// Check if prompt is already injected
	for _, injected := range session.InjectedPrompts {
		if injected == promptName {
			return &InjectPromptResponse{
				Success: false,
				Message: fmt.Sprintf("Prompt '%s' is already injected in session '%s'", promptName, sessionID),
			}, nil
		}
	}

	// Get prompt handler
	handler, exists := promptHandlers[promptName]
	if !exists {
		return &InjectPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Prompt '%s' not found", promptName),
		}, nil
	}

	// Call handler to get prompt content
	ctx := context.Background()
	request := mcp.GetPromptRequest{}
	result, err := handler(ctx, request)
	if err != nil {
		return &InjectPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Error getting prompt content: %v", err),
		}, err
	}

	// Add prompt to session context
	session.Context = append(session.Context, result.Messages...)
	session.InjectedPrompts = append(session.InjectedPrompts, promptName)
	session.LastActivity = time.Now()

	return &InjectPromptResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully injected '%s' prompt into session '%s'", promptName, sessionID),
		Content: result,
	}, nil
}

// GetSession retrieves a session by ID (read-only)
func (sm *SessionManager) GetSession(sessionID string) (*SessionContext, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[sessionID]
	return session, exists
}

// RemoveSession removes a session
func (sm *SessionManager) RemoveSession(sessionID string) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if _, exists := sm.sessions[sessionID]; exists {
		delete(sm.sessions, sessionID)
		return true
	}
	return false
}

// ListSessions returns all session IDs
func (sm *SessionManager) ListSessions() []string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	sessions := make([]string, 0, len(sm.sessions))
	for sessionID := range sm.sessions {
		sessions = append(sessions, sessionID)
	}
	return sessions
}

// CleanupInactiveSessions removes sessions that haven't been active for the specified duration
func (sm *SessionManager) CleanupInactiveSessions(maxAge time.Duration) int {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for sessionID, session := range sm.sessions {
		if session.LastActivity.Before(cutoff) {
			delete(sm.sessions, sessionID)
			removed++
		}
	}

	return removed
}
