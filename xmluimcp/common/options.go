package common

//import "github.com/mikeschinkel/go-cliutil"

// Options implements the Options() interface method
var _ interface{ Options() } = (*Options)(nil)

// Options contains runtime options for the MCP server
type Options struct {
	XMLUIDir      string   // Path to XMLUI source directory (empty = auto-download)
	ExampleRoot   string   // Root directory for examples (optional)
	ExampleDirs   []string // Subdirectories within example root (optional)
	AnalyticsFile string   // Path to analytics file (optional)
	//Verbosity     cliutil.Verbosity // Output verbosity level
	//Quiet         bool              // Suppress non-error output
}

// Options implements the Options() marker interface
func (Options) Options() {}
