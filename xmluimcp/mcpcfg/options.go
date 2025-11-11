package mcpcfg

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/mikeschinkel/go-cliutil"
)

const (
	DefaultQuiet         = false
	DefaultVerbosity     = cliutil.DefaultVerbosity
	DefaultXMLUIDir      = ""
	DefaultExampleRoot   = ""
	DefaultAnalyticsFile = ""
)

type Options struct {
	XMLUIDir      string
	ExampleRoot   string
	ExampleDirs   []string
	AnalyticsFile string
	Quiet         bool
	Verbosity     int
}

func (*Options) Options() {}

type OptionsArgs struct {
	XMLUIDir      *string
	ExampleRoot   *string
	ExampleDirs   []string
	AnalyticsFile *string
	Quiet         *bool
	Verbosity     *int
}

func NewOptions(args OptionsArgs) *Options {
	opts := &Options{}

	if args.XMLUIDir != nil {
		opts.XMLUIDir = *args.XMLUIDir
	} else {
		opts.XMLUIDir = DefaultXMLUIDir
	}

	if args.ExampleRoot != nil {
		opts.ExampleRoot = *args.ExampleRoot
	} else {
		opts.ExampleRoot = DefaultExampleRoot
	}

	if args.ExampleDirs != nil {
		opts.ExampleDirs = args.ExampleDirs
	}

	if args.AnalyticsFile != nil {
		opts.AnalyticsFile = *args.AnalyticsFile
	} else {
		opts.AnalyticsFile = DefaultAnalyticsFile
	}

	if args.Quiet != nil {
		opts.Quiet = *args.Quiet
	} else {
		opts.Quiet = DefaultQuiet
	}

	if args.Verbosity != nil {
		opts.Verbosity = *args.Verbosity
	} else {
		opts.Verbosity = DefaultVerbosity
	}

	return opts
}

// OptionsFlagSet encapsulates flag parsing for MCP server options
type OptionsFlagSet struct {
	fs            *flag.FlagSet
	xmluiDir      *string
	exampleRoot   *string
	exampleDirs   stringSliceFlag
	analyticsFile *string
	quiet         *bool
	verbosity     *int
}

// NewOptionsFlagSet creates a new OptionsFlagSet with all flags configured
func NewOptionsFlagSet(name string) *OptionsFlagSet {
	ofs := &OptionsFlagSet{
		fs:            flag.NewFlagSet(name, flag.ContinueOnError),
		xmluiDir:      new(string),
		exampleRoot:   new(string),
		exampleDirs:   stringSliceFlag{},
		analyticsFile: new(string),
		quiet:         new(bool),
		verbosity:     new(int),
	}

	ofs.fs.StringVar(ofs.xmluiDir, "xmlui-dir", DefaultXMLUIDir, "Path to XMLUI source directory (empty = auto-download)")
	ofs.fs.StringVar(ofs.exampleRoot, "example-root", DefaultExampleRoot, "Root directory for examples (optional)")
	ofs.fs.Var(&ofs.exampleDirs, "example-dir", "Example subdirectory (can be specified multiple times)")
	ofs.fs.StringVar(ofs.analyticsFile, "analytics", DefaultAnalyticsFile, "Path to analytics file (optional)")

	ofs.fs.BoolVar(ofs.quiet, "quiet", DefaultQuiet, "Disable display of most command line output")
	ofs.fs.BoolVar(ofs.quiet, "q", DefaultQuiet, "Disable display of most command line output (shorthand)")

	ofs.fs.IntVar(ofs.verbosity, "verbosity", DefaultVerbosity, "Verbosity of command line output (1 to 3, default 1)")
	ofs.fs.IntVar(ofs.verbosity, "v", DefaultVerbosity, "Verbosity of command line output (shorthand)")

	return ofs
}

// FlagSet returns the underlying flag.FlagSet for composition with other CLIs
func (ofs *OptionsFlagSet) FlagSet() *flag.FlagSet {
	return ofs.fs
}

// Parse parses the provided arguments and returns the constructed Options
func (ofs *OptionsFlagSet) Parse(args []string) error {
	return ofs.fs.Parse(args)
}

// Options constructs and returns the Options from parsed flag values
func (ofs *OptionsFlagSet) Options() (opts *Options, err error) {
	var verbosity cliutil.Verbosity

	verbosity, err = cliutil.ParseVerbosity(*ofs.verbosity)
	if err != nil {
		goto end
	}

	opts = NewOptions(OptionsArgs{
		XMLUIDir:      ofs.xmluiDir,
		ExampleRoot:   ofs.exampleRoot,
		ExampleDirs:   ofs.exampleDirs.values(),
		AnalyticsFile: ofs.analyticsFile,
		Quiet:         ofs.quiet,
		Verbosity:     intPtr(int(verbosity)),
	})

end:
	return opts, err
}

var options *Options

func GetOptions() (opts *Options, err error) {
	var ofs *OptionsFlagSet

	if options != nil {
		opts = options
		goto end
	}

	ofs = NewOptionsFlagSet(os.Args[0])

	// Set custom flag usage
	ofs.fs.Usage = func() {
		fprintf(ofs.fs.Output(), "Usage of %s:\n", os.Args[0])
		ofs.fs.VisitAll(func(f *flag.Flag) {
			prefix := "-"
			if len(f.Name) > 1 {
				prefix = "--"
			}
			fprintf(ofs.fs.Output(), "  %s%s: %s\n", prefix, f.Name, f.Usage)
		})
	}

	err = ofs.Parse(os.Args[1:])
	if err != nil {
		goto end
	}

	opts, err = ofs.Options()
	if err != nil {
		goto end
	}

	options = opts
end:
	return opts, err
}

func intPtr(n int) *int {
	return &n
}

type stringSliceFlag []string

func (f *stringSliceFlag) values() (v []string) {
	v = make([]string, 0, len(*f))
	for _, s := range *f {
		v = append(v, strings.TrimSpace(s))
	}
	return v
}

func (f *stringSliceFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringSliceFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func fprintf(w io.Writer, format string, a ...any) {
	_, err := fmt.Fprintf(w, format, a...)
	if err != nil {
		log.Printf("ERROR: %v", err)
	}
}
