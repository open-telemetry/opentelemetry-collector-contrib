// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)

type schemaProcessor struct {
	telemetry component.TelemetrySettings
	config    *Config

	log *zap.Logger

	manager           translation.Manager
	telemetryBuilder  *metadata.TelemetryBuilder
	migrationFromURLs map[string]string // target schema URL → migration from URL (for metrics)
}

func newSchemaProcessor(_ context.Context, conf component.Config, set processor.Settings) (*schemaProcessor, error) {
	cfg, ok := conf.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration provided")
	}

	migrationMap := make(map[string]*translation.Version)
	migrationFromURLs := make(map[string]string)
	for _, entry := range cfg.Migration {
		_, fromVersion, err := translation.GetFamilyAndVersion(entry.From)
		if err != nil {
			return nil, fmt.Errorf("migration.from: %w", err)
		}
		migrationMap[entry.Target] = fromVersion
		migrationFromURLs[entry.Target] = entry.From
	}

	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	m, err := translation.NewManager(
		cfg.Targets,
		set.Logger.Named("schema-manager"),
		cfg.CacheCooldown,
		cfg.CacheRetryLimit,
		telemetryBuilder,
		migrationMap,
	)
	if err != nil {
		return nil, err
	}
	return &schemaProcessor{
		config:            cfg,
		telemetry:         set.TelemetrySettings,
		log:               set.Logger,
		manager:           m,
		telemetryBuilder:  telemetryBuilder,
		migrationFromURLs: migrationFromURLs,
	}, nil
}

func (t schemaProcessor) recordTranslation(ctx context.Context, fromSchemaURL, toSchemaURL string) {
	attrs := []attribute.KeyValue{
		attribute.String("from_schema_url", fromSchemaURL),
		attribute.String("to_schema_url", toSchemaURL),
	}
	if migrationFrom, ok := t.migrationFromURLs[toSchemaURL]; ok {
		attrs = append(attrs, attribute.String("migration_from_schema_url", migrationFrom))
	}
	t.telemetryBuilder.ProcessorSchemaTranslated.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (t schemaProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	var skipped, failed int64
	for _, rLogs := range ld.ResourceLogs().All() {
		resourceSchemaURL := rLogs.SchemaUrl()
		if resourceSchemaURL != "" {
			t.log.Debug("requesting translation for resourceSchemaURL", zap.String("resourceSchemaURL", resourceSchemaURL))
			tr, err := t.manager.RequestTranslation(ctx, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				t.telemetryBuilder.ProcessorSchemaResourceFailed.Add(ctx, 1)
				return ld, err
			}
			err = tr.ApplyAllResourceChanges(rLogs, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to apply resource changes", zap.Error(err))
				t.telemetryBuilder.ProcessorSchemaResourceFailed.Add(ctx, 1)
				return ld, err
			}
			t.recordTranslation(ctx, resourceSchemaURL, tr.TargetSchemaURL())
		}
		for _, logs := range rLogs.ScopeLogs().All() {
			schemaURL := cmp.Or(logs.SchemaUrl(), resourceSchemaURL)
			if schemaURL == "" {
				skipped++
				continue
			}
			tr, err := t.manager.RequestTranslation(ctx, schemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				failed++
				continue
			}
			err = tr.ApplyScopeLogChanges(logs, schemaURL)
			if err != nil {
				t.log.Error("failed to apply scope log changes", zap.Error(err))
				failed++
			}
		}
	}
	if skipped > 0 {
		t.telemetryBuilder.ProcessorSchemaLogsSkipped.Add(ctx, skipped)
	}
	if failed > 0 {
		t.telemetryBuilder.ProcessorSchemaLogsFailed.Add(ctx, failed)
	}
	return ld, nil
}

func (t schemaProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	var skipped, failed int64
	for _, rMetric := range md.ResourceMetrics().All() {
		resourceSchemaURL := rMetric.SchemaUrl()
		if resourceSchemaURL != "" {
			t.log.Debug("requesting translation for resourceSchemaURL", zap.String("resourceSchemaURL", resourceSchemaURL))
			tr, err := t.manager.RequestTranslation(ctx, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				t.telemetryBuilder.ProcessorSchemaResourceFailed.Add(ctx, 1)
				return md, err
			}
			err = tr.ApplyAllResourceChanges(rMetric, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to apply resource changes", zap.Error(err))
				t.telemetryBuilder.ProcessorSchemaResourceFailed.Add(ctx, 1)
				return md, err
			}
			t.recordTranslation(ctx, resourceSchemaURL, tr.TargetSchemaURL())
		}
		for _, metric := range rMetric.ScopeMetrics().All() {
			schemaURL := cmp.Or(metric.SchemaUrl(), resourceSchemaURL)
			if schemaURL == "" {
				skipped++
				continue
			}
			tr, err := t.manager.RequestTranslation(ctx, schemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				failed++
				continue
			}
			err = tr.ApplyScopeMetricChanges(metric, schemaURL)
			if err != nil {
				t.log.Error("failed to apply scope metric changes", zap.Error(err))
				failed++
			}
		}
	}
	if skipped > 0 {
		t.telemetryBuilder.ProcessorSchemaMetricsSkipped.Add(ctx, skipped)
	}
	if failed > 0 {
		t.telemetryBuilder.ProcessorSchemaMetricsFailed.Add(ctx, failed)
	}
	return md, nil
}

func (t schemaProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	var skipped, failed int64
	for _, rTrace := range td.ResourceSpans().All() {
		resourceSchemaURL := rTrace.SchemaUrl()
		if resourceSchemaURL != "" {
			t.log.Debug("requesting translation for resourceSchemaURL", zap.String("resourceSchemaURL", resourceSchemaURL))
			tr, err := t.manager.RequestTranslation(ctx, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				t.telemetryBuilder.ProcessorSchemaResourceFailed.Add(ctx, 1)
				return td, err
			}
			err = tr.ApplyAllResourceChanges(rTrace, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to apply resource changes", zap.Error(err))
				t.telemetryBuilder.ProcessorSchemaResourceFailed.Add(ctx, 1)
				return td, err
			}
			t.recordTranslation(ctx, resourceSchemaURL, tr.TargetSchemaURL())
		}
		for _, span := range rTrace.ScopeSpans().All() {
			schemaURL := cmp.Or(span.SchemaUrl(), resourceSchemaURL)
			if schemaURL == "" {
				skipped++
				continue
			}
			tr, err := t.manager.RequestTranslation(ctx, schemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				failed++
				continue
			}
			err = tr.ApplyScopeSpanChanges(span, schemaURL)
			if err != nil {
				t.log.Error("failed to apply scope span changes", zap.Error(err))
				failed++
			}
		}
	}
	if skipped > 0 {
		t.telemetryBuilder.ProcessorSchemaTracesSkipped.Add(ctx, skipped)
	}
	if failed > 0 {
		t.telemetryBuilder.ProcessorSchemaTracesFailed.Add(ctx, failed)
	}
	return td, nil
}

// start will add HTTP provider to the manager and prefetch schemas
func (t *schemaProcessor) start(ctx context.Context, host component.Host) error {
	for _, entry := range t.config.Migration {
		t.log.Warn("migration is enabled: attribute renames will preserve original attributes alongside renamed ones; this is a temporary migration aid",
			zap.String("target", entry.Target), zap.String("from", entry.From))
	}
	client, err := t.config.ToClient(ctx, host.GetExtensions(), t.telemetry)
	if err != nil {
		return err
	}
	t.manager.AddProvider(translation.NewHTTPProvider(client))

	wg := new(errgroup.Group)
	for _, schemaURL := range t.config.Prefetch {
		t.log.Info("prefetching schema", zap.String("url", schemaURL))
		wg.Go(func() error {
			if _, err := t.manager.RequestTranslation(ctx, schemaURL); err != nil {
				t.log.Warn("failed to prefetch schema", zap.String("url", schemaURL), zap.Error(err))
			}
			return nil
		})
	}

	return wg.Wait()
}
