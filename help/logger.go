// Package help holds the small utilities the analyzer ships alongside the
// tokenizer: character predicates, a sleep helper, and the logger factory the
// Elasticsearch plugin used to attach a prefix to every record.
package help

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Level is the severity of a log record.
type Level string

// The levels the analyzer emits.
const (
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// defaultOutput is where records go until SetOutput says otherwise.
var defaultOutput io.Writer = os.Stderr

var (
	outputMu sync.Mutex
	output   = defaultOutput
)

// SetOutput redirects every logger. The original delegated this choice to the
// log4j2 configuration Elasticsearch supplies; a Go library has to expose it.
func SetOutput(w io.Writer) {
	outputMu.Lock()
	defer outputMu.Unlock()
	output = w
}

// Logger writes records under a logger name, optionally tagged with the prefix
// marker PrefixPluginLogger attached in the original.
type Logger struct {
	name   string
	prefix string
}

// markers interns prefixes so two loggers built with the same prefix share one
// marker, the way PrefixPluginLogger's WeakHashMap did.
var (
	markersMu sync.Mutex
	markers   = map[string]string{}
)

func markersSize() int {
	markersMu.Lock()
	defer markersMu.Unlock()
	return len(markers)
}

// GetLogger returns a logger with no prefix marker.
func GetLogger(name string) *Logger { return GetLoggerWithPrefix("", name) }

// GetLoggerWithPrefix returns a logger whose records carry a prefix marker. An
// empty prefix yields a plain logger, exactly as ESPluginLoggerFactory did.
func GetLoggerWithPrefix(prefix, name string) *Logger {
	if prefix == "" {
		return &Logger{name: name}
	}
	markersMu.Lock()
	if _, ok := markers[prefix]; !ok {
		markers[prefix] = prefix
	}
	markersMu.Unlock()
	return &Logger{name: name, prefix: prefix}
}

// Prefix returns the name of this logger's marker.
func (l *Logger) Prefix() string { return l.prefix }

// Name returns the logger name.
func (l *Logger) Name() string { return l.name }

// Info logs at INFO. Parameters replace the "{}" placeholders in message, the
// way log4j2's parameterized messages do.
func (l *Logger) Info(message string, params ...any) { l.log(LevelInfo, message, params...) }

// Warn logs at WARN.
func (l *Logger) Warn(message string, params ...any) { l.log(LevelWarn, message, params...) }

// Error logs at ERROR.
func (l *Logger) Error(message string, params ...any) { l.log(LevelError, message, params...) }

func (l *Logger) log(level Level, message string, params ...any) {
	var b strings.Builder
	b.WriteString(string(level))
	b.WriteByte(' ')
	b.WriteString(l.name)
	if l.prefix != "" {
		b.WriteString(" [")
		b.WriteString(l.prefix)
		b.WriteByte(']')
	}
	b.WriteByte(' ')
	b.WriteString(Format(message, params...))
	b.WriteByte('\n')

	outputMu.Lock()
	defer outputMu.Unlock()
	_, _ = io.WriteString(output, b.String())
}

// Format substitutes params for the "{}" placeholders in message, the way a
// log4j2 parameterized message does. A leftover trailing error is rendered
// after the message as the record's throwable; any other leftover parameter is
// dropped, which is also what log4j2 does with one.
func Format(message string, params ...any) string {
	var b strings.Builder
	next := 0
	for {
		at := strings.Index(message, "{}")
		if at < 0 || next >= len(params) {
			break
		}
		b.WriteString(message[:at])
		b.WriteString(render(params[next]))
		next++
		message = message[at+2:]
	}
	b.WriteString(message)
	if next < len(params) {
		if err, ok := params[len(params)-1].(error); ok {
			b.WriteString(" ")
			b.WriteString(err.Error())
		}
	}
	return b.String()
}

func render(v any) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}
	return fmt.Sprint(v)
}
