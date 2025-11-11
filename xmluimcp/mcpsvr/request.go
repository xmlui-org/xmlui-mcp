package mcpsvr

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// BoolPtr returns a pointer to a bool value.
func BoolPtr(b bool) *bool {
	return &b
}

// RequestArgument extracts a string argument from a CallToolRequest.
// Returns empty string if the argument doesn't exist or isn't a string.
// The returned string is automatically trimmed of leading/trailing whitespace.
func RequestArgument(req mcp.CallToolRequest, name string) string {
	if req.Params.Arguments == nil {
		return ""
	}
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		return ""
	}
	value, ok := args[name]
	if !ok {
		return ""
	}
	var str string
	if s, ok := value.(string); ok {
		str = s
	} else {
		str = fmt.Sprintf("%v", value)
	}
	return strings.TrimSpace(str)
}

// RequestArguments extracts the full arguments map from a CallToolRequest.
// Returns nil if arguments cannot be converted to a map.
func RequestArguments(req mcp.CallToolRequest) map[string]interface{} {
	if req.Params.Arguments == nil {
		return nil
	}
	args, ok := req.Params.Arguments.(map[string]interface{})
	if !ok {
		return nil
	}
	return args
}
