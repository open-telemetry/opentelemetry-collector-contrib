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

	attrs := r.collectSpanAttributes(resource, scope, span)
	return r.resolveIndexName(cfg.TracesIndex, cfg.TracesIndexFallback, cfg.TracesIndexTimeFormat, attrs, timestamp)
}

// ResolveLogRecordIndex resolves the logs index name per log record using placeholders, fallback, and time format
func (r *indexResolver) ResolveLogRecordIndex(cfg *Config, resource pcommon.Resource, scope pcommon.InstrumentationScope, logRecord plog.LogRecord, timestamp time.Time) string {
	if cfg.LogsIndex == "" {
		indexName := getIndexName(cfg.Dataset, cfg.Namespace, "ss4o_logs")
		return r.appendTimeFormat(indexName, cfg.LogsIndexTimeFormat, timestamp)
	}

	attrs := r.collectLogRecordAttributes(resource, scope, logRecord)
	return r.resolveIndexName(cfg.LogsIndex, cfg.LogsIndexFallback, cfg.LogsIndexTimeFormat, attrs, timestamp)
}

// collectSpanAttributes extracts resource, scope, and span attributes into a flat map for placeholder resolution with precedence
func (*indexResolver) collectSpanAttributes(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) map[string]string {
	attrs := make(map[string]string)
	// Resource attributes (lowest precedence)
	resAttrs := resource.Attributes()
	resAttrs.Range(func(k string, v pcommon.Value) bool {
		attrs[k] = v.AsString()
		return true
	})
	// Instrumentation scope attributes
	if scope.Name() != "" {
		attrs["scope.name"] = scope.Name()
	}
	if scope.Version() != "" {
		attrs["scope.version"] = scope.Version()
	}
	scopeAttrs := scope.Attributes()
	scopeAttrs.Range(func(k string, v pcommon.Value) bool {
		attrs[k] = v.AsString()
		return true
	})
	// Span attributes (highest precedence, overwrites)
	spanAttrs := span.Attributes()
	spanAttrs.Range(func(k string, v pcommon.Value) bool {
		attrs[k] = v.AsString()
		return true
	})
	return attrs
}

// collectLogRecordAttributes extracts resource, scope, and log record attributes into a flat map for placeholder resolution with precedence
func (*indexResolver) collectLogRecordAttributes(resource pcommon.Resource, scope pcommon.InstrumentationScope, logRecord plog.LogRecord) map[string]string {
	attrs := make(map[string]string)
	// Resource attributes (lowest precedence)
	resAttrs := resource.Attributes()
	resAttrs.Range(func(k string, v pcommon.Value) bool {
		attrs[k] = v.AsString()
		return true
	})
	// Instrumentation scope attributes
	if scope.Name() != "" {
		attrs["scope.name"] = scope.Name()
	}
	if scope.Version() != "" {
		attrs["scope.version"] = scope.Version()
	}
	scopeAttrs := scope.Attributes()
	scopeAttrs.Range(func(k string, v pcommon.Value) bool {
		attrs[k] = v.AsString()
		return true
	})
	// Log record attributes (highest precedence, overwrites)
	logAttrs := logRecord.Attributes()
	logAttrs.Range(func(k string, v pcommon.Value) bool {
		attrs[k] = v.AsString()
		return true
	})
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
