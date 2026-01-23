package logger

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is a structured JSON logger.
type Logger struct {
	out   io.Writer
	mu    sync.Mutex
	level Level
}

// LogEntry represents a single log line.
type LogEntry struct {
	Level   string                 `json:"level"`
	Time    string                 `json:"time"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// New creates a new Logger.
func New(out io.Writer, level Level) *Logger {
	return &Logger{
		out:   out,
		level: level,
	}
}

// Default returns a standard stdout logger.
func Default() *Logger {
	return New(os.Stdout, INFO)
}

func (l *Logger) log(level Level, msg string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Level:   level.String(),
		Time:    time.Now().Format(time.RFC3339),
		Message: msg,
		Fields:  fields,
	}

	// Encode to JSON - In v2 optimize with byte buffer pool
	// For now, minimal allocation optimization is just reusing the encoder logic?
	// Actually json.Marshal is fine for now.

	l.mu.Lock()
	defer l.mu.Unlock()

	json.NewEncoder(l.out).Encode(entry)
}

// Info logs an info message.
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, msg, f)
}

// Error logs an error message.
func (l *Logger) Error(msg string, err error) {
	l.log(ERROR, msg, map[string]interface{}{"error": err.Error()})
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, msg, f)
}
