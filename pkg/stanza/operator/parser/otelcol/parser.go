// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/otelcol"

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// consoleSeparator is zap's field separator for console encoding. It is a tab
// and not a space, which is what makes the optional logger/caller fields
// separable from message text that contains spaces.
const consoleSeparator = "\t"

// callerRegex matches zap's caller encoding, "<trimmed path>/<file>.go:<line>".
// Matching on shape is needed because the caller field is optional and sits
// directly in front of the message.
var callerRegex = regexp.MustCompile(`^\S+\.go:\d+$`)

// Parser is an operator that parses otelcol self-logs and promotes the resource field onto the entry's Resource.
type Parser struct {
	helper.ParserOperator
}

// ProcessBatch will process a batch of log entries.
func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWithCallback(ctx, entries, parseLine, p.postProcess)
}

// Process will process a single log entry.
func (p *Parser) Process(ctx context.Context, e *entry.Entry) error {
	return p.ProcessWithCallback(ctx, e, parseLine, p.postProcess)
}

// parseLine normalizes a json- or console-encoded otelcol self-log line into a flat map.
func parseLine(value any) (any, error) {
	var line string
	switch v := value.(type) {
	case string:
		line = v
	case []byte:
		line = string(v)
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as an otelcol self-log", value)
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return nil, errors.New("empty line cannot be parsed as an otelcol self-log")
	}

	if isJSONEncoded(line) {
		return parseJSONLine(line)
	}
	return parseConsoleLine(line)
}

// isJSONEncoded reports whether a line uses zap's json encoding rather than its
// console encoding. Console lines always start with a timestamp, so a leading
// "{" is unambiguous.
func isJSONEncoded(line string) bool {
	return strings.HasPrefix(line, "{")
}

// parseJSONLine parses a line where the entire line is a single JSON object.
func parseJSONLine(line string) (map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return nil, fmt.Errorf("line cannot be parsed as a json-encoded otelcol self-log: %w", err)
	}
	return parsed, nil
}

// parseConsoleLine parses zap's console encoding, whose fields are laid out as
//
//	<ts> <level> [<logger>] [<caller>] <msg> [<fields>]
//
// separated by consoleSeparator, optionally followed by a stacktrace. Logger
// and caller are absent when the logger is unnamed and when
// service::telemetry::logs::disable_caller is set, so both are recognized by
// position and shape rather than assumed to be there.
func parseConsoleLine(line string) (map[string]any, error) {
	// The stacktrace is appended after the fields on its own lines, so it is
	// only seen here when the entry was recombined upstream.
	line, stacktrace, hasStacktrace := strings.Cut(line, "\n")

	parts := strings.Split(line, consoleSeparator)
	if len(parts) < 3 {
		return nil, errors.New("line cannot be parsed as a console-encoded otelcol self-log")
	}

	result := map[string]any{
		"ts":    parts[0],
		"level": parts[1],
	}
	if hasStacktrace {
		result["stacktrace"] = strings.TrimSpace(stacktrace)
	}

	rest := parts[2:]
	if len(rest) > 1 {
		if fields, ok := decodeConsoleFields(rest[len(rest)-1]); ok {
			maps.Copy(result, fields)
			rest = rest[:len(rest)-1]
		}
	}

	switch {
	case len(rest) > 1 && callerRegex.MatchString(rest[0]):
		result["caller"] = rest[0]
		rest = rest[1:]
	case len(rest) > 2 && callerRegex.MatchString(rest[1]):
		result["logger"] = rest[0]
		result["caller"] = rest[1]
		rest = rest[2:]
	case len(rest) > 1 && rest[0] != "" && !strings.Contains(rest[0], " "):
		// Only reachable with disable_caller set and a named logger. A logger
		// name never contains a space, a message that was split by a tab of
		// its own almost always does.
		result["logger"] = rest[0]
		rest = rest[1:]
	}

	// A message containing consoleSeparator is spread over the remaining parts.
	result["msg"] = strings.Join(rest, consoleSeparator)

	return result, nil
}

// decodeConsoleFields reports whether the last console field is zap's
// structured context object rather than the tail of the message.
func decodeConsoleFields(field string) (map[string]any, bool) {
	if !strings.HasPrefix(field, "{") || !strings.HasSuffix(field, "}") {
		return nil, false
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(field), &fields); err != nil {
		return nil, false
	}
	return fields, true
}

// postProcess promotes the ts, level, msg, and resource fields of a parsed otelcol self-log
// onto the entry's Timestamp, Severity, Body, and Resource.
func (p *Parser) postProcess(e *entry.Entry) error {
	val, ok := e.Get(p.ParseTo)
	if !ok {
		return nil
	}
	m, ok := val.(map[string]any)
	if !ok {
		return nil
	}

	tsErr := setTimestamp(e, m)
	setSeverity(e, m)
	setBody(e, m)
	setResource(e, m)

	return tsErr
}

// setTimestamp promotes "ts" onto the entry's Timestamp. The key is only
// removed once it has been understood, so a value the parser can't interpret
// stays in the attributes instead of being dropped on the floor.
func setTimestamp(e *entry.Entry, m map[string]any) error {
	tsVal, ok := m["ts"]
	if !ok {
		return nil
	}

	v, ok := tsVal.(string)
	if !ok {
		return fmt.Errorf("timestamp parser: unsupported type %T for \"ts\" field", tsVal)
	}

	// The collector pins zap to ISO8601 for both encodings, so this is the only
	// layout a self-log can carry.
	t, err := time.Parse("2006-01-02T15:04:05.999Z0700", v)
	if err != nil {
		return fmt.Errorf("timestamp parser: value %q is not an ISO8601 timestamp", v)
	}
	e.Timestamp = t
	delete(m, "ts")
	return nil
}

func setSeverity(e *entry.Entry, m map[string]any) {
	lvlVal, ok := m["level"]
	if !ok {
		return
	}
	defer delete(m, "level")

	lvlStr, ok := lvlVal.(string)
	if !ok {
		return
	}
	e.SeverityText = lvlStr

	switch strings.ToLower(lvlStr) {
	case "trace":
		e.Severity = entry.Trace
	case "debug":
		e.Severity = entry.Debug
	case "info":
		e.Severity = entry.Info
	case "warn", "warning":
		e.Severity = entry.Warn
	case "error":
		e.Severity = entry.Error
	case "dpanic", "panic", "fatal":
		e.Severity = entry.Fatal
	}
}

func setBody(e *entry.Entry, m map[string]any) {
	msgVal, ok := m["msg"]
	if !ok {
		return
	}
	e.Body = msgVal
	delete(m, "msg")
}

func setResource(e *entry.Entry, m map[string]any) {
	resVal, ok := m["resource"]
	if !ok {
		return
	}
	defer delete(m, "resource")

	resMap, ok := resVal.(map[string]any)
	if !ok {
		return
	}
	if e.Resource == nil {
		e.Resource = make(map[string]any)
	}
	maps.Copy(e.Resource, resMap)
}
