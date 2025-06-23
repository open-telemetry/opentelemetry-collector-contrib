// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type logExporter struct {
	client       *opensearch.Client
	Index        string
	bulkAction   string
	model        mappingModel
	httpSettings confighttp.ClientConfig
	telemetry    component.TelemetrySettings
	config       *Config // add config reference
}

func newLogExporter(cfg *Config, set exporter.Settings) *logExporter {
	model := &encodeModel{
		dedup:             cfg.Dedup,
		dedot:             cfg.Dedot,
		sso:               cfg.Mode == MappingSS4O.String(),
		flattenAttributes: cfg.Mode == MappingFlattenAttributes.String(),
		timestampField:    cfg.TimestampField,
		unixTime:          cfg.UnixTimestamp,
		dataset:           cfg.Dataset,
		namespace:         cfg.Namespace,
	}

	return &logExporter{
		telemetry:    set.TelemetrySettings,
		Index:        getIndexName(cfg.Dataset, cfg.Namespace, cfg.LogsIndex),
		bulkAction:   cfg.BulkAction,
		httpSettings: cfg.ClientConfig,
		model:        model,
		config:       cfg, // set config
	}
}

func (l *logExporter) Start(ctx context.Context, host component.Host) error {
	httpClient, err := l.httpSettings.ToClient(ctx, host, l.telemetry)
	if err != nil {
		return err
	}

	client, err := newOpenSearchClient(l.httpSettings.Endpoint, httpClient, l.telemetry.Logger)
	if err != nil {
		return err
	}

	l.client = client
	return nil
}

func (l *logExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	indexer := newLogBulkIndexer("", l.bulkAction, l.model)
	startErr := indexer.start(l.client)
	if startErr != nil {
		return startErr
	}

	// Index name resolution
	// If LogsIndex is not set, use the default index name, otherwise resolve it using the config and attributes.
	// This allows for dynamic index naming based on log attributes.
	var indexName string
	if l.config.LogsIndex == "" {
		indexName = l.Index
	} else {
		// Collect attributes from resource/log record
		attrs := collectAttributes(ld)
		logTimestamp := time.Now() // Replace with actual log timestamp extraction
		indexName = resolveLogIndexName(l.config, attrs, logTimestamp)
	}
	indexer.index = indexName
	indexer.submit(ctx, ld)
	indexer.close(ctx)
	return indexer.joinedError()
}

// collectAttributes extracts resource and log record attributes into a flat map for placeholder resolution.
func collectAttributes(ld plog.Logs) map[string]string {
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

// resolveLogIndexName resolves the logs index name using placeholders, fallback, and time format.
func resolveLogIndexName(cfg *Config, attrs map[string]string, t time.Time) string {
	index := cfg.LogsIndex
	placeholderPattern := regexp.MustCompile(`%\{([^}]+)\}`)
	index = placeholderPattern.ReplaceAllStringFunc(index, func(match string) string {
		key := placeholderPattern.FindStringSubmatch(match)[1]
		if val, ok := attrs[key]; ok && val != "" {
			return val
		}
		if cfg.LogsIndexFallback != "" {
			return cfg.LogsIndexFallback
		}
		return "unknown" // default if placeholder not found
	})
	if cfg.LogsIndexTimeFormat != "" {
		index = index + "-" + t.Format(convertGoTimeFormat(cfg.LogsIndexTimeFormat))
	}
	return index
}

// convertGoTimeFormat converts a Java-style date format to Go's time format.
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

func getIndexName(dataset, namespace, index string) string {
	if len(index) != 0 {
		return index
	}

	return strings.Join([]string{"ss4o_logs", dataset, namespace}, "-")
}
