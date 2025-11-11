package xmluimcp

import (
	"github.com/mikeschinkel/go-cliutil"
	"github.com/mikeschinkel/go-dt"
	"github.com/xmlui-org/mcpsvr/xmluimcp/common"
	"github.com/xmlui-org/mcpsvr/xmluimcp/mcpcfg"

	. "github.com/mikeschinkel/go-doterr"
)

// ParseOptions converts raw options from mcpcfg.Options into
// validated common.Options with embedded cliutil.GlobalOptions.
func ParseOptions(cfgOpts *mcpcfg.Options) (opts *common.Options, err error) {
	var errs []error
	var cliOpts *cliutil.GlobalOptions
	var xmluiDir dt.DirPath
	var exampleRoot dt.DirPath
	var exampleDirs []dt.DirPath
	var analyticsFile dt.Filepath

	cliOpts, err = cliutil.NewGlobsalOptions(cliutil.GlobalOptionsArgs{
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

	opts = &common.Options{
		GlobalOptions: cliOpts,
		XMLUIDir:      xmluiDir,
		ExampleRoot:   exampleRoot,
		ExampleDirs:   exampleDirs,
		AnalyticsFile: analyticsFile,
	}

	return opts, CombineErrs(errs)
}
