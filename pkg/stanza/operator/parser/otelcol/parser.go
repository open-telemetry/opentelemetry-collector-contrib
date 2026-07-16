// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/otelcol"

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	formatAuto    = "auto"
	formatJSON    = "json"
	formatConsole = "console"

	// malformedTrailingFieldsAttr is set on the entry's leftover attributes
	// whenever a console-encoded line ended with what looked like an attempt
	// at structured trailing fields (i.e. the line ends in "}"), but the
	// trailing text never resolved into valid JSON.
	malformedTrailingFieldsAttr = "otelcol.self_log.malformed_trailing_json"
)

// consoleLinePrefixRegex matches zap's console encoding up through the message:
// <ts> <level> [<caller>] <rest>. It does not attempt to locate any trailing
// JSON fields - regex alone can't tell whether a "{" is real JSON or just
// part of the message text (e.g. "Config value for pool {default}").
var consoleLinePrefixRegex = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(?:(\S+:\d+)\s+)?(.*)$`)

// Parser is an operator that parses otelcol self-logs and promotes the resource field onto the entry's Resource.
type Parser struct {
	helper.ParserOperator

	format string
}

// ProcessBatch will process a batch of log entries.
func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWithCallback(ctx, entries, p.parse, p.postProcess)
}

// Process will process a single log entry.
func (p *Parser) Process(ctx context.Context, e *entry.Entry) error {
	return p.ProcessWithCallback(ctx, e, p.parse, p.postProcess)
}

// parse normalizes a json- or console-encoded otelcol self-log line into a flat map.
func (p *Parser) parse(value any) (any, error) {
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

	format := p.format
	if format == formatAuto {
		format = detectFormat(line)
	}

	switch format {
	case formatJSON:
		return parseJSONLine(line)
	case formatConsole:
		return parseConsoleLine(line)
	default:
		return nil, fmt.Errorf("unknown otelcol self-log format %q", format)
	}
}

// detectFormat determines whether a line is json- or console-encoded.
func detectFormat(line string) string {
	if strings.HasPrefix(line, "{") {
		return formatJSON
	}
	return formatConsole
}

// parseJSONLine parses a line where the entire line is a single JSON object.
func parseJSONLine(line string) (map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return nil, fmt.Errorf("line cannot be parsed as a json-encoded otelcol self-log: %w", err)
	}
	return parsed, nil
}

// parseConsoleLine parses a line where ts/level/caller are plain text, followed
// by a message that may or may not have structured JSON fields appended after it.
func parseConsoleLine(line string) (map[string]any, error) {
	match := consoleLinePrefixRegex.FindStringSubmatch(line)
	if match == nil {
		return nil, errors.New("line cannot be parsed as a console-encoded otelcol self-log")
	}

	msg, fields, malformed := splitConsoleMessageAndFields(match[4])

	result := map[string]any{
		"ts":    match[1],
		"level": match[2],
		"msg":   msg,
	}
	if caller := match[3]; caller != "" {
		result["caller"] = caller
	}
	if malformed {
		result[malformedTrailingFieldsAttr] = true
	}
	maps.Copy(result, fields)

	return result, nil
}

// splitConsoleMessageAndFields separates a message from an optional trailing
// JSON object, using a real JSON decoder rather than a regex, so that:
//   - braces inside the message text (e.g. "pool {default} is missing") don't
//     get mistaken for the start of structured fields, and
//   - nested objects inside the real trailing fields (e.g. "resource":{...})
//     are matched to their correct closing brace, not the first "}" found.

func splitConsoleMessageAndFields(rest string) (msg string, fields map[string]any, malformed bool) {
	rest = strings.TrimSpace(rest)

	looksLikeTrailingJSON := strings.HasSuffix(rest, "}")

	sawUnresolvedBrace := false
	searchFrom := 0
	for {
		idx := strings.IndexByte(rest[searchFrom:], '{')
		if idx == -1 {
			break
		}
		idx += searchFrom

		candidate := rest[idx:]
		dec := json.NewDecoder(strings.NewReader(candidate))
		var decoded map[string]any
		if err := dec.Decode(&decoded); err == nil {
			if strings.TrimSpace(candidate[dec.InputOffset():]) == "" {
				return strings.TrimSpace(rest[:idx]), decoded, false
			}
		}

		if looksLikeTrailingJSON {
			sawUnresolvedBrace = true
		}
		searchFrom = idx + 1
	}

	return rest, nil, sawUnresolvedBrace
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

	setTimestamp(e, m)
	setSeverity(e, m)
	setBody(e, m)
	setResource(e, m)

	return nil
}

func setTimestamp(e *entry.Entry, m map[string]any) {
	tsVal, ok := m["ts"]
	if !ok {
		return
	}
	defer delete(m, "ts")

	switch v := tsVal.(type) {
	case string:
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05.999Z0700",
			"2006-01-02 15:04:05.999999999",
		} {
			if t, err := time.Parse(layout, v); err == nil {
				e.Timestamp = t
				return
			}
		}
	case float64:
		sec, dec := math.Modf(v)
		e.Timestamp = time.Unix(int64(sec), int64(math.Round(dec*1e6))*1e3)
	case int64:
		e.Timestamp = time.Unix(v, 0)
	}
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