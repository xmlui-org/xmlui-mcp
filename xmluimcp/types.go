package xmluimcp

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// PromptInfo represents basic prompt information
type PromptInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PromptContent represents full prompt content with messages
type PromptContent struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Messages    []mcp.PromptMessage `json:"messages"`
}

// PromptHandler defines the signature for prompt handlers
type PromptHandler func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

// SessionContext represents a session with injected prompts
type SessionContext struct {
	ID              string              `json:"id"`
	InjectedPrompts []string            `json:"injected_prompts"`
	LastActivity    time.Time           `json:"last_activity"`
	Context         []mcp.PromptMessage `json:"context"`
}

// InjectPromptRequest represents a request to inject a prompt
type InjectPromptRequest struct {
	SessionID  string `json:"session_id"`
	PromptName string `json:"prompt_name"`
}

// InjectPromptResponse represents the response from injecting a prompt
type InjectPromptResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Content *mcp.GetPromptResult `json:"content,omitempty"`
}

// StartupInfo represents server startup information
type StartupInfo struct {
	Type       string         `json:"type"`
	Prompts    []PromptInfo   `json:"prompts"`
	Tools      []ToolInfo     `json:"tools"`
	XmluiRules XmluiRulesInfo `json:"xmlui_rules"`
}

// ToolInfo represents basic tool information
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// XmluiRulesInfo represents XMLUI rules information
type XmluiRulesInfo struct {
	Description string `json:"description"`
	Content     string `json:"content"`
}
