package xmluihelpers

import (
	"errors"
	"log/slog"
	"os"

	"github.com/mikeschinkel/go-cfgstore"
	"github.com/mikeschinkel/go-cliutil"
	"github.com/mikeschinkel/go-doterr"
	"github.com/mikeschinkel/go-dt"
	"github.com/mikeschinkel/go-logutil"
)

var ErrFailedWriterSetup = errors.New("failed writer setup")
var ErrFailedLoggerSetup = errors.New("failed logger setup")

type WriterLoggerArgs struct {
	Quiet      bool
	Verbosity  cliutil.Verbosity
	ConfigSlug dt.PathSegment
	LogFile    dt.Filename
}

func CreateWriterLogger(args *WriterLoggerArgs) (wr cliutil.WriterLogger, err error) {
	var configDir dt.DirPath
	var logger *slog.Logger
	var logDir dt.DirPath

	writer := cliutil.NewWriter(&cliutil.WriterArgs{
		Quiet:     args.Quiet,
		Verbosity: args.Verbosity,
	})

	configDir, err = cfgstore.CLIConfigDir(args.ConfigSlug)
	if err != nil {
		err = doterr.NewErr(
			ErrFailedWriterSetup,
			cfgstore.ErrFailedGettingUserConfigDir,
			err,
		)
		goto end
	}
	logDir = dt.DirPathJoin(configDir, "logs")

	logger, err = createLogger(logDir, writer, args)
	if err != nil {
		goto end
	}
	wr = cliutil.NewWriterLogger(writer, logger)

end:
	return wr, err
}

func createLogger(logDir dt.DirPath, writer cliutil.Writer, args *WriterLoggerArgs) (logger *slog.Logger, err error) {
	var tmpFile *os.File
	var canWrite bool
	var fp dt.Filepath

	canWrite, _ = logDir.CanWrite()
	if !canWrite {
		tmpDir := dt.TempDir()
		tmpFile, err = dt.CreateTemp(tmpDir, string(args.ConfigSlug)+"-*")
		writer.Errorf("Cannot write to %s. Logging to %s-* instead\n", logDir, tmpDir, args.ConfigSlug)
		if err != nil {
			err = doterr.NewErr(dt.ErrFailedtoCreateTempFile, err)
			goto end
		}
		defer dt.CloseOrLog(tmpFile)
		logDir = dt.DirPath(tmpFile.Name())
	}
	err = logDir.MkdirAll(0755)
	if err != nil {
		err = doterr.NewErr(dt.ErrFailedToMakeDirectory,
			"log_dir", logDir,
			err)
		goto end
	}
	fp = dt.FilepathJoin(logDir, args.LogFile)
	logger, err = logutil.CreateJSONFileLogger(fp)
	if err != nil {
		err = doterr.NewErr(dt.ErrFailedtoCreateFile,
			"log_file", fp,
			err,
		)
		goto end
	}
	cfgstore.SetLogger(logger)
end:
	if err != nil {
		err = doterr.WithErr(err, ErrFailedLoggerSetup)
	}
	return logger, err
}
