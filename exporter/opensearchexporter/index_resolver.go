// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// indexResolver handles dynamic index name resolution for logs and traces
type indexResolver struct {
	placeholderPattern *regexp.Regexp
}

// newIndexResolver creates a new index resolver instance
func newIndexResolver() *indexResolver {
	return &indexResolver{
		placeholderPattern: regexp.MustCompile(`%\{([^}]+)\}`),
	}
}

// extractPlaceholderKeys extracts unique placeholder keys from the index pattern
func (r *indexResolver) extractPlaceholderKeys(template string) []string {
	matches := r.placeholderPattern.FindAllStringSubmatch(template, -1)
	keySet := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			keySet[match[1]] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	return keys
}

// resolveIndexName handles the common logic for resolving index names with placeholders
func (r *indexResolver) resolveIndexName(indexPattern, fallback, timeFormat string, attrs map[string]string, timestamp time.Time) string {
	index := r.placeholderPattern.ReplaceAllStringFunc(indexPattern, func(match string) string {
		key := r.placeholderPattern.FindStringSubmatch(match)[1]
		if val, ok := attrs[key]; ok && val != "" {
			return val
		}
		if fallback != "" {
			return fallback
		}
		return "unknown"
	})

	return r.appendTimeFormat(index, timeFormat, timestamp)
}

// appendTimeFormat appends time suffix if format is specified
func (*indexResolver) appendTimeFormat(index, timeFormat string, timestamp time.Time) string {
	if timeFormat != "" {
		return index + "-" + timestamp.Format(convertGoTimeFormat(timeFormat))
	}
	return index
}

// ResolveSpanIndex resolves the traces index name per span using placeholders, fallback, and time format
func (r *indexResolver) ResolveSpanIndex(cfg *Config, resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span, timestamp time.Time) string {
	if cfg.TracesIndex == "" {
		indexName := getIndexName(cfg.Dataset, cfg.Namespace, "ss4o_traces")
		return r.appendTimeFormat(indexName, cfg.TracesIndexTimeFormat, timestamp)
	}

	keys := r.extractPlaceholderKeys(cfg.TracesIndex)
	attrs := r.collectSpanAttributes(resource, scope, span, keys)
	return r.resolveIndexName(cfg.TracesIndex, cfg.TracesIndexFallback, cfg.TracesIndexTimeFormat, attrs, timestamp)
}

// ResolveLogRecordIndex resolves the logs index name per log record using placeholders, fallback, and time format
func (r *indexResolver) ResolveLogRecordIndex(cfg *Config, resource pcommon.Resource, scope pcommon.InstrumentationScope, logRecord plog.LogRecord, timestamp time.Time) string {
	if cfg.LogsIndex == "" {
		indexName := getIndexName(cfg.Dataset, cfg.Namespace, "ss4o_logs")
		return r.appendTimeFormat(indexName, cfg.LogsIndexTimeFormat, timestamp)
	}

	keys := r.extractPlaceholderKeys(cfg.LogsIndex)
	attrs := r.collectLogRecordAttributes(resource, scope, logRecord, keys)
	return r.resolveIndexName(cfg.LogsIndex, cfg.LogsIndexFallback, cfg.LogsIndexTimeFormat, attrs, timestamp)
}

// collectSpanAttributes extracts resource, scope, and span attributes into a flat map for placeholder resolution with precedence
func (*indexResolver) collectSpanAttributes(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span, keys []string) map[string]string {
	attrs := make(map[string]string, len(keys))
	for _, key := range keys {
		if key == "scope.name" {
			if scope.Name() != "" {
				attrs[key] = scope.Name()
			}
		} else if key == "scope.version" {
			if scope.Version() != "" {
				attrs[key] = scope.Version()
			}
		} else {
			if v, ok := span.Attributes().Get(key); ok {
				attrs[key] = v.AsString()
			} else if v, ok := scope.Attributes().Get(key); ok {
				attrs[key] = v.AsString()
			} else if v, ok := resource.Attributes().Get(key); ok {
				attrs[key] = v.AsString()
			}
		}
	}
	return attrs
}

// collectLogRecordAttributes extracts resource, scope, and log record attributes into a flat map for placeholder resolution with precedence
func (*indexResolver) collectLogRecordAttributes(resource pcommon.Resource, scope pcommon.InstrumentationScope, logRecord plog.LogRecord, keys []string) map[string]string {
	attrs := make(map[string]string, len(keys))
	for _, key := range keys {
		if key == "scope.name" {
			if scope.Name() != "" {
				attrs[key] = scope.Name()
			}
		} else if key == "scope.version" {
			if scope.Version() != "" {
				attrs[key] = scope.Version()
			}
		} else {
			if v, ok := logRecord.Attributes().Get(key); ok {
				attrs[key] = v.AsString()
			} else if v, ok := scope.Attributes().Get(key); ok {
				attrs[key] = v.AsString()
			} else if v, ok := resource.Attributes().Get(key); ok {
				attrs[key] = v.AsString()
			}
		}
	}
	return attrs
}

// convertGoTimeFormat converts a Java-style date format to Go's time format
func convertGoTimeFormat(format string) string {
	// Support yyyy, yy, MM, dd, HH, mm, ss -> 2006, 06, 01, 02, 15, 04, 05
	f := format
	f = strings.ReplaceAll(f, "yyyy", "2006")
	f = strings.ReplaceAll(f, "yy", "06")
	f = strings.ReplaceAll(f, "MM", "01")
	f = strings.ReplaceAll(f, "dd", "02")
	f = strings.ReplaceAll(f, "HH", "15")
	f = strings.ReplaceAll(f, "mm", "04")
	f = strings.ReplaceAll(f, "ss", "05")
	return f
}

// getIndexName provides default index naming for backward compatibility
func getIndexName(dataset, namespace, defaultPrefix string) string {
	return strings.Join([]string{defaultPrefix, dataset, namespace}, "-")
}
