// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/otelcol"

import (
	"context"
	"fmt"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses Collector self-logs.
type Parser struct {
	helper.ParserOperator
}

// ProcessBatch will process a batch of log entries.
func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWithCallback(ctx, entries, p.parse, p.postProcess)
}

// Process will process a single log entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWithCallback(ctx, entry, p.parse, p.postProcess)
}

// parse will parse a value as JSON.
func (*Parser) parse(value any) (any, error) {
	var parsedValue map[string]any
	switch m := value.(type) {
	case string:
		err := json.Unmarshal([]byte(m), &parsedValue)
		if err != nil {
			return nil, err
		}
	case []byte:
		err := json.Unmarshal(m, &parsedValue)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as JSON", value)
	}

	return parsedValue, nil
}

// postProcess performs the specific mappings for otelcol self-logs
func (p *Parser) postProcess(e *entry.Entry) error {
	val, ok := e.Get(p.ParseTo)
	if !ok {
		return nil
	}

	parsedMap, ok := val.(map[string]any)
	if !ok {
		return nil
	}

	// 1. Parse Timestamp (ts)
	if tsVal, ok := parsedMap["ts"]; ok {
		switch v := tsVal.(type) {
		case string:
			for _, layout := range []string{
				time.RFC3339Nano,
				time.RFC3339,
				"2006-01-02 15:04:05.999999999",
			} {
				if t, err := time.Parse(layout, v); err == nil {
					e.Timestamp = t
					break
				}
			}
		case float64:
			sec, dec := math.Modf(v)
			nanos := int64(math.Round(dec*1e6)) * 1e3
			e.Timestamp = time.Unix(int64(sec), nanos)
		case int64:
			e.Timestamp = time.Unix(v, 0)
		}
		delete(parsedMap, "ts")
	}

	// 2. Parse Severity (level)
	if lvlVal, ok := parsedMap["level"]; ok {
		if lvlStr, ok := lvlVal.(string); ok {
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
			case "dpanic", "panic":
				e.Severity = entry.Fatal
			case "fatal":
				e.Severity = entry.Fatal
			}
			e.SeverityText = lvlStr
		}
		delete(parsedMap, "level")
	}

	// 3. Parse Body (msg)
	if msgVal, ok := parsedMap["msg"]; ok {
		e.Body = msgVal
		delete(parsedMap, "msg")
	}

	// 4. Parse Resource (resource)
	if resVal, ok := parsedMap["resource"]; ok {
		if resMap, ok := resVal.(map[string]any); ok {
			if e.Resource == nil {
				e.Resource = make(map[string]any)
			}
			maps.Copy(e.Resource, resMap)
		}
		delete(parsedMap, "resource")
	}

	return nil
}
