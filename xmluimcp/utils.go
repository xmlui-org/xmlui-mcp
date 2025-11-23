package xmluimcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mikeschinkel/go-cliutil"
	"github.com/mikeschinkel/go-dt"
)

// printStartupInfo prints server startup information as JSON to stderr
func printStartupInfo(prompts []mcp.Prompt, tools []mcp.Tool, xmluiRulesHandler PromptHandler, logger *slog.Logger) {
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
		logger.Error("Error marshaling startup info", "error", err)
		return
	}
	{
		var tmpFile *os.File
		tmpFile, err = dt.CreateTemp(dt.TempDir(), "xmlui-mcp-startupinfo-*.json")
		if err != nil {
			logger.Warn("Error creating temp file to write XMLUI MCP Startup info", "error", err)
			return
		}
		defer closeOrLog(tmpFile, logger)
		_, err = tmpFile.Write(jsonBytes)
		if err != nil {
			logOnError(err, logger)
			cliutil.Stderrf("ERROR: Failed to write XMLUI MCP Startup info to %s; %v", tmpFile.Name(), err)
			goto end
		}
		cliutil.Stderrf("XMLUI MCP Startup info written to %s\n\n", tmpFile.Name())
	end:
		return
	}

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
