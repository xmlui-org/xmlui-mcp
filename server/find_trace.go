package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewFindTraceTool() (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("xmlui_find_trace",
		mcp.WithDescription(
			"Finds the most recent XMLUI trace export. "+
				"Returns the file path so Claude can read it with the Read tool. "+
				"Trace files match the pattern xs-trace-*.json. "+
				"Searches ~/Downloads by default; use the dir parameter to override."),
		mcp.WithString("dir",
			mcp.Description("Directory to search for trace files. Defaults to ~/Downloads."),
		),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dir, _ := req.Params.Arguments["dir"].(string)
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get home directory: %v", err)), nil
			}
			dir = filepath.Join(home, "Downloads")
		}

		pattern := filepath.Join(dir, "xs-trace-*.json")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to glob for trace files: %v", err)), nil
		}

		if len(matches) == 0 {
			return mcp.NewToolResultError("No trace files found in ~/Downloads matching xs-trace-*.json"), nil
		}

		// Sort by modification time, newest first
		sort.Slice(matches, func(i, j int) bool {
			infoI, errI := os.Stat(matches[i])
			infoJ, errJ := os.Stat(matches[j])
			if errI != nil || errJ != nil {
				return false
			}
			return infoI.ModTime().After(infoJ.ModTime())
		})

		newest := matches[0]
		info, err := os.Stat(newest)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to stat %s: %v", newest, err)), nil
		}

		result := fmt.Sprintf("Latest trace: %s\nSize: %d bytes\nModified: %s",
			newest, info.Size(), info.ModTime().Format("2006-01-02 15:04:05"))

		return mcp.NewToolResultText(result), nil
	}

	return tool, handler
}
