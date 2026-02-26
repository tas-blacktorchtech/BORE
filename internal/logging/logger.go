package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// logLevel represents the severity of a log message.
type logLevel int

const (
	levelDebug logLevel = iota
	levelInfo
	levelWarn
	levelError
)

// String returns the human-readable tag for the level (e.g. "DEBUG").
func (l logLevel) String() string {
	switch l {
	case levelDebug:
		return "DEBUG"
	case levelInfo:
		return "INFO"
	case levelWarn:
		return "WARN"
	case levelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// parseLevel converts a string level name to the internal logLevel value.
// Unrecognised strings default to levelInfo.
func parseLevel(s string) logLevel {
	switch strings.ToLower(s) {
	case "debug":
		return levelDebug
	case "info":
		return levelInfo
	case "warn":
		return levelWarn
	case "error":
		return levelError
	default:
		return levelInfo
	}
}

// Logger is a thread-safe, level-filtered logger that writes timestamped
// lines to a file and optionally mirrors them to stderr. Before every write
// it checks whether the underlying file has exceeded its size threshold and,
// if so, triggers a rotation.
type Logger struct {
	mu        sync.Mutex
	filePath  string
	file      *os.File
	logger    *log.Logger
	level     atomic.Int32
	maxBytes  int64
	toConsole bool
}

// NewLogger opens (or creates) the log file at path and returns a ready
// Logger. level is one of "debug", "info", "warn", "error". rotationMB
// is the maximum file size in megabytes before rotation occurs. When
// toConsole is true every message is also printed to stderr.
func NewLogger(path string, level string, rotationMB int, toConsole bool) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("logging: open %s: %w", path, err)
	}

	l := &Logger{
		filePath:  path,
		file:      f,
		logger:    log.New(f, "", 0), // we format our own prefix
		maxBytes:  int64(rotationMB) * 1024 * 1024,
		toConsole: toConsole,
	}
	l.level.Store(int32(parseLevel(level)))
	return l, nil
}

// Debug logs a message at DEBUG level.
func (l *Logger) Debug(msg string, args ...any) {
	l.logMsg(levelDebug, msg, args...)
}

// Info logs a message at INFO level.
func (l *Logger) Info(msg string, args ...any) {
	l.logMsg(levelInfo, msg, args...)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(msg string, args ...any) {
	l.logMsg(levelWarn, msg, args...)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(msg string, args ...any) {
	l.logMsg(levelError, msg, args...)
}

// logMsg is the shared implementation behind Debug/Info/Warn/Error.
func (l *Logger) logMsg(lvl logLevel, msg string, args ...any) {
	if lvl < logLevel(l.level.Load()) {
		return
	}

	// Format the message if args were supplied.
	text := msg
	if len(args) > 0 {
		text = fmt.Sprintf(msg, args...)
	}

	line := time.Now().Format(time.RFC3339) + " [" + lvl.String() + "] " + text

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check size and rotate if necessary.
	l.checkRotate()

	l.logger.Output(0, line)

	if l.toConsole {
		fmt.Fprintln(os.Stderr, line)
	}
}

// checkRotate stats the current log file and, if it exceeds the configured
// threshold, performs a rotation. Must be called with l.mu held.
func (l *Logger) checkRotate() {
	info, err := l.file.Stat()
	if err != nil {
		return
	}
	if info.Size() < l.maxBytes {
		return
	}

	// Rotate first — rename the file on disk. The open file handle remains
	// valid on Unix even after the rename.
	if err := rotate(l.filePath); err != nil {
		fmt.Fprintf(os.Stderr, "logging: rotation failed: %v\n", err)
		return
	}

	// Open a fresh log file at the original path.
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: reopen after rotation failed: %v\n", err)
		return
	}

	// Only now close the old handle and swap in the new one.
	old := l.file
	l.file = f
	l.logger.SetOutput(f)
	old.Close()
}

// Close flushes and closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
