package logging

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// Level is a log severity level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "debug",
	LevelInfo:  "info",
	LevelWarn:  "warn",
	LevelError: "error",
}

// Logger is a minimal structured logger.
type Logger struct {
	level Level
	out   io.Writer
}

// NewLogger creates a logger writing JSON lines to out.
func NewLogger(out io.Writer, level Level) *Logger {
	return &Logger{level: level, out: out}
}

func (l *Logger) min(level Level) bool {
	return level >= l.level
}

func (l *Logger) write(level Level, fields map[string]interface{}) {
	if !l.min(level) {
		return
	}
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["level"] = levelNames[level]
	fields["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(fields)
	if err != nil {
		b = []byte(`{"level":"error","msg":"failed to marshal log entry"}`)
	}
	b = append(b, '\n')
	if _, err := l.out.Write(b); err != nil {
		// Best effort; avoid recursion.
	}
}

// Debug writes a debug event.
func (l *Logger) Debug(fields map[string]interface{}) { l.write(LevelDebug, fields) }

// Info writes an info event.
func (l *Logger) Info(fields map[string]interface{}) { l.write(LevelInfo, fields) }

// Warn writes a warn event.
func (l *Logger) Warn(fields map[string]interface{}) { l.write(LevelWarn, fields) }

// Error writes an error event.
func (l *Logger) Error(fields map[string]interface{}) { l.write(LevelError, fields) }

// Stdout returns a logger writing to stdout.
func Stdout(level Level) *Logger {
	return NewLogger(os.Stdout, level)
}
