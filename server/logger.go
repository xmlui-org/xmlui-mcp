package server

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const LogFileName = "xmlui-mcp-server.log"

var (
	appLogger   *log.Logger
	appLogFile  *os.File
	appLogMutex sync.RWMutex
)

// InitLogger initializes the global logger.
// It takes a directory path where the log file will be created.
func InitLogger(logDir string) error {
	appLogMutex.Lock()
	defer appLogMutex.Unlock()

	// Close existing file if open
	if appLogFile != nil {
		appLogFile.Close()
	}

	logPath := filepath.Join(logDir, LogFileName)

	var err error
	appLogFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Fallback to stderr if we can't open the file
		appLogger = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
		return fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	appLogger = log.New(appLogFile, "", log.LstdFlags|log.Lmicroseconds)
	return nil
}

// Logger returns the global logger instance.
// It returns a stderr logger if InitLogger hasn't been called.
func Logger() *log.Logger {
	appLogMutex.RLock()
	defer appLogMutex.RUnlock()

	if appLogger == nil {
		return log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	}
	return appLogger
}

// WriteDebugLog writes a formatted message to the log.
// It uses the configured logger or falls back to stderr.
func WriteDebugLog(format string, args ...interface{}) {
	appLogMutex.RLock()
	defer appLogMutex.RUnlock()

	logger := appLogger
	if logger == nil {
		log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds).Printf(format, args...)
		return
	}

	logger.Printf(format, args...)

	// Flush immediately to ensure it's written, useful for debugging
	if appLogFile != nil {
		appLogFile.Sync()
	}
}

// CloseLogger closes the log file resource.
func CloseLogger() {
	appLogMutex.Lock()
	defer appLogMutex.Unlock()

	if appLogFile != nil {
		appLogFile.Close()
		appLogFile = nil
	}
	appLogger = nil
}
