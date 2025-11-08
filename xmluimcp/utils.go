package xmluimcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/xmlui-org/mcp/xmluimcp/mcpsvr"

	"github.com/mark3labs/mcp-go/mcp"
)

// getCurrentDir returns the current working directory
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// printStartupInfo prints server startup information as JSON to stderr
func printStartupInfo(prompts []mcp.Prompt, tools []mcp.Tool, xmluiRulesHandler PromptHandler) {
	// Convert prompts to API format
	var promptInfoList []PromptInfo
	for _, prompt := range prompts {
		promptInfoList = append(promptInfoList, PromptInfo{
			Name:        prompt.Name,
			Description: prompt.Description,
		})
	}

	// Convert tools to API format
	var toolInfoList []ToolInfo
	for _, tool := range tools {
		toolInfoList = append(toolInfoList, ToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema.Properties,
		})
	}

	// Get XMLUI rules content
	var xmluiRulesContent string
	if xmluiRulesHandler != nil {
		ctx := context.Background()
		request := mcp.GetPromptRequest{}
		result, err := xmluiRulesHandler(ctx, request)
		if err == nil && len(result.Messages) > 0 {
			if textContent, ok := result.Messages[0].Content.(*mcp.TextContent); ok {
				xmluiRulesContent = textContent.Text
			} else if textContent, ok := result.Messages[0].Content.(mcp.TextContent); ok {
				xmluiRulesContent = textContent.Text
			}
		}
	}

	// Create startup info
	startupInfo := StartupInfo{
		Type:    "xmlui-mcp",
		Prompts: promptInfoList,
		Tools:   toolInfoList,
		XmluiRules: XmluiRulesInfo{
			Description: "Essential rules and guidelines for XMLUI development",
			Content:     xmluiRulesContent,
		},
	}

	// Print as JSON to stderr
	jsonBytes, err := json.MarshalIndent(startupInfo, "", "  ")
	if err != nil {
		mcpsvr.WriteDebugLog("Error marshaling startup info: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "%s\n", string(jsonBytes))
}

// getToolInfo creates ToolInfo from mcp.Tool
func getToolInfo(tool mcp.Tool) ToolInfo {
	return ToolInfo{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: tool.InputSchema.Properties,
	}
}

// getXmluiRulesContent extracts XMLUI rules content from the prompt handler
func getXmluiRulesContent(handler PromptHandler) string {
	if handler == nil {
		return ""
	}

	ctx := context.Background()
	request := mcp.GetPromptRequest{}
	result, err := handler(ctx, request)
	if err != nil {
		return ""
	}

	if len(result.Messages) == 0 {
		return ""
	}

	switch content := result.Messages[0].Content.(type) {
	case *mcp.TextContent:
		return content.Text
	case mcp.TextContent:
		return content.Text
	default:
		return ""
	}
}
