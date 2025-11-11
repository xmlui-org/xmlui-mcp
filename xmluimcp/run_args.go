package xmluimcp

import (
	"github.com/xmlui-org/mcpsvr/xmluimcp/common"
)

// RunArgs contains all the configuration and dependencies needed to run the MCP server.
// This struct is used to pass configuration from the CLI layer to the core server logic,
// matching the pattern used by xmluisvr for consistency.
type RunArgs struct {
	CLIArgs []string        // Command-line arguments (for reference)
	Config  *common.Config  // Typed runtime configuration
	Options *common.Options // Parsed command-line options (embedded in Config, kept for compatibility)
}
