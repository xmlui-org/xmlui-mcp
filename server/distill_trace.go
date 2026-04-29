package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewDistillTraceTool() (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("xmlui_distill_trace",
		mcp.WithDescription(
			"Distills an exported XMLUI Inspector trace into a structured per-step "+
				"user-journey summary in JSON. Each step describes one user action "+
				"(click, fill, dblclick, keydown, navigation, etc.) with the API calls "+
				"it triggered, value changes, toasts, modals, validation errors, and "+
				"state diffs. Use this output to narrate what happened in the trace, "+
				"diagnose layout/behavior issues, or generate Playwright-style replays. "+
				"If no path is given, finds the most recent xs-trace-*.json in "+
				"~/Downloads."),
		mcp.WithString("path",
			mcp.Description("Absolute path to a trace JSON file. If omitted, uses the most recent xs-trace-*.json in ~/Downloads."),
		),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, _ := req.Params.Arguments["path"].(string)
		if path == "" {
			resolved, err := latestTraceFile()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			path = resolved
		}

		exe, err := os.Executable()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("could not resolve xmlui binary: %v", err)), nil
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}

		cmd := exec.CommandContext(ctx, exe, "distill-trace", path)
		out, err := cmd.Output()
		if err != nil {
			stderr := ""
			if exitErr, ok := err.(*exec.ExitError); ok {
				stderr = strings.TrimSpace(string(exitErr.Stderr))
			}
			return mcp.NewToolResultError(fmt.Sprintf("distill-trace failed: %v\n%s", err, stderr)), nil
		}

		// Strip the "Using most recent trace: ..." preamble that the CLI writes
		// to stderr. (Output() does not include stderr, so this is just a
		// safety belt against any incidental stdout noise that might appear in
		// future versions.)
		jsonStart := strings.IndexByte(string(out), '{')
		if jsonStart > 0 {
			out = out[jsonStart:]
		}

		return mcp.NewToolResultText(string(out)), nil
	}

	return tool, handler
}

func latestTraceFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	dir := filepath.Join(home, "Downloads")
	pattern := filepath.Join(dir, "xs-trace-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob %s: %w", pattern, err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no xs-trace-*.json files found in %s", dir)
	}
	sort.Slice(matches, func(i, j int) bool {
		infoI, errI := os.Stat(matches[i])
		infoJ, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})
	return matches[0], nil
}
