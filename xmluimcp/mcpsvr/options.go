package mcpsvr

import (
	"github.com/mikeschinkel/go-cliutil"
	"github.com/mikeschinkel/go-dt"
)

// Options implements the Options() interface method
var _ interface{ Options() } = (*Options)(nil)

// Options contains parsed, strongly-typed runtime options for the MCP server.
// It embeds *cliutil.GlobalOptions to inherit standard CLI behaviors like Quiet() and Verbosity().
type Options struct {
	*cliutil.GlobalOptions
	XMLUIDir      dt.DirPath   // Path to XMLUI source directory (empty = auto-download)
	ExampleRoot   dt.DirPath   // Root directory for examples (optional)
	ExampleDirs   []dt.DirPath // Subdirectories within example root (optional)
	AnalyticsFile dt.Filepath  // Path to analytics file (optional)
}
