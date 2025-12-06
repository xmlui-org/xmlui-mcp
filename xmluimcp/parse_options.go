package xmluimcp

import (
	"github.com/mikeschinkel/go-cliutil"
	"github.com/mikeschinkel/go-dt"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/mcpcfg"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/mcpsvr"
)

// ParseOptions converts raw options from mcpcfg.Options into
// validated mcpsvr.Options with embedded cliutil.GlobalOptions.
func ParseOptions(cfgOpts *mcpcfg.Options) (opts *mcpsvr.Options, err error) {
	var errs []error
	var globalOpts *cliutil.GlobalOptions
	var xmluiDir dt.DirPath
	var exampleRoot dt.DirPath
	var exampleDirs []dt.DirPath
	var analyticsFile dt.Filepath

	globalOpts, err = cliutil.NewGlobalOptions(cliutil.GlobalOptionsArgs{
		Quiet:     &cfgOpts.Quiet,
		Verbosity: &cfgOpts.Verbosity,
	})
	errs = AppendErr(errs, err)

	xmluiDir, err = dt.ParseDirPath(cfgOpts.XMLUIDir)
	errs = AppendErr(errs, err)

	exampleRoot, err = dt.ParseDirPath(cfgOpts.ExampleRoot)
	errs = AppendErr(errs, err)

	exampleDirs, err = dt.ParseDirPaths(cfgOpts.ExampleDirs)
	errs = AppendErr(errs, err)

	analyticsFile, err = dt.ParseFilepath(cfgOpts.AnalyticsFile)
	errs = AppendErr(errs, err)

	opts = &mcpsvr.Options{
		GlobalOptions: globalOpts,
		XMLUIDir:      xmluiDir,
		ExampleRoot:   exampleRoot,
		ExampleDirs:   exampleDirs,
		AnalyticsFile: analyticsFile,
	}

	return opts, CombineErrs(errs)
}
