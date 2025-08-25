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

// ResolveLogIndex resolves the logs index name using placeholders, fallback, and time format
func (r *indexResolver) ResolveLogIndex(cfg *Config, ld plog.Logs, timestamp time.Time) string {
	if cfg.LogsIndex == "" {
		// Use default pattern
		indexName := getIndexName(cfg.Dataset, cfg.Namespace, "ss4o_logs")
		return r.appendTimeFormat(indexName, cfg.LogsIndexTimeFormat, timestamp)
	}

	attrs := r.collectLogAttributes(ld)
	return r.resolveIndexName(cfg.LogsIndex, cfg.LogsIndexFallback, cfg.LogsIndexTimeFormat, attrs, timestamp)
}

// ResolveTraceIndex resolves the traces index name using placeholders, fallback, and time format
func (r *indexResolver) ResolveTraceIndex(cfg *Config, td ptrace.Traces, timestamp time.Time) string {
	if cfg.TracesIndex == "" {
		// Use default pattern
		indexName := getIndexName(cfg.Dataset, cfg.Namespace, "ss4o_traces")
		return r.appendTimeFormat(indexName, cfg.TracesIndexTimeFormat, timestamp)
	}

	attrs := r.collectTraceAttributes(td)
	return r.resolveIndexName(cfg.TracesIndex, cfg.TracesIndexFallback, cfg.TracesIndexTimeFormat, attrs, timestamp)
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

// collectLogAttributes extracts resource and log record attributes into a flat map for placeholder resolution
func (*indexResolver) collectLogAttributes(ld plog.Logs) map[string]string {
	attrs := make(map[string]string)
	resLogsSlice := ld.ResourceLogs()
	for i := 0; i < resLogsSlice.Len(); i++ {
		resLogs := resLogsSlice.At(i)
		// Resource attributes
		resAttrs := resLogs.Resource().Attributes()
		resAttrs.Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})
		logSlice := resLogs.ScopeLogs()
		for j := 0; j < logSlice.Len(); j++ {
			scopeLogs := logSlice.At(j)
			// Instrumentation scope attributes
			if scope := scopeLogs.Scope(); scope.Name() != "" {
				attrs["scope.name"] = scope.Name()
			}
			if scope := scopeLogs.Scope(); scope.Version() != "" {
				attrs["scope.version"] = scope.Version()
			}
			scopeAttrs := scopeLogs.Scope().Attributes()
			scopeAttrs.Range(func(k string, v pcommon.Value) bool {
				attrs[k] = v.AsString()
				return true
			})
			logs := scopeLogs.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				logAttrs := logs.At(k).Attributes()
				logAttrs.Range(func(k string, v pcommon.Value) bool {
					attrs[k] = v.AsString()
					return true
				})
			}
		}
	}
	return attrs
}

// collectTraceAttributes extracts resource and span attributes into a flat map for placeholder resolution
func (*indexResolver) collectTraceAttributes(td ptrace.Traces) map[string]string {
	attrs := make(map[string]string)
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resSpan := resourceSpans.At(i)
		// Resource attributes
		resAttrs := resSpan.Resource().Attributes()
		resAttrs.Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})
		scopeSpans := resSpan.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scopeSpan := scopeSpans.At(j)
			// Instrumentation scope attributes
			if scope := scopeSpan.Scope(); scope.Name() != "" {
				attrs["scope.name"] = scope.Name()
			}
			if scope := scopeSpan.Scope(); scope.Version() != "" {
				attrs["scope.version"] = scope.Version()
			}
			scopeAttrs := scopeSpan.Scope().Attributes()
			scopeAttrs.Range(func(k string, v pcommon.Value) bool {
				attrs[k] = v.AsString()
				return true
			})
			spans := scopeSpan.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				spanAttrs := span.Attributes()
				spanAttrs.Range(func(k string, v pcommon.Value) bool {
					attrs[k] = v.AsString()
					return true
				})
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
