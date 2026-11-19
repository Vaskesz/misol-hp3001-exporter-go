package main

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sync"
    "time"
)

type LogLevel int

const (
    InfoLevel LogLevel = iota
    WarnLevel
    ErrorLevel
)

var (
    logMu     sync.Mutex
    logFile   *os.File
    logWriter io.Writer
)

// initLogger opens the log file (if path is non-empty) and configures the
// output writer. Parent directories are created automatically if missing.
// Returns an error if the file or its directory cannot be created/opened.
func initLogger(path string) error {
    if path == "" {
        logWriter = os.Stdout
        return nil
    }

    // Resolve to absolute path so log entries show the real location,
    // and so that MkdirAll works correctly regardless of CWD changes.
    absPath, err := filepath.Abs(path)
    if err != nil {
        return fmt.Errorf("cannot resolve path '%s': %w", path, err)
    }

    // Create parent directories (e.g. "logs") if they do not exist.
    // filepath.Dir("error.log") == ".", and MkdirAll(".") is a safe no-op.
    dir := filepath.Dir(absPath)
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return fmt.Errorf("cannot create log directory '%s': %w", dir, err)
    }

    f, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
    if err != nil {
        return fmt.Errorf("cannot open log file '%s': %w", absPath, err)
    }
    logFile = f
    logWriter = io.MultiWriter(os.Stdout, f)
    return nil
}

func logMessage(level LogLevel, format string, args ...interface{}) {
    logMu.Lock()
    defer logMu.Unlock()

    var levelStr string
    switch level {
    case InfoLevel:
        levelStr = "INFO"
    case WarnLevel:
        levelStr = "WARNING"
    case ErrorLevel:
        levelStr = "ERROR"
    default:
        levelStr = "UNKNOWN"
    }

    msg := fmt.Sprintf(format, args...)
    fmt.Fprintf(logWriter, "%s: %s: %s\n",
        time.Now().Format("2006-01-02 15:04:05"), levelStr, msg)
}

func logInfo(format string, args ...interface{}) {
    logMessage(InfoLevel, format, args...)
}

func logWarn(format string, args ...interface{}) {
    logMessage(WarnLevel, format, args...)
}

func logError(format string, args ...interface{}) {
    logMessage(ErrorLevel, format, args...)
}

// closeLogger flushes and closes the log file, if it was opened.
func closeLogger() {
    if logFile != nil {
        _ = logFile.Close()
    }
}