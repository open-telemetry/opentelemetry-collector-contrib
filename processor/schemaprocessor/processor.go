// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"cmp"
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)

type schemaProcessor struct {
	telemetry component.TelemetrySettings
	config    *Config

	log *zap.Logger

	manager translation.Manager

	logsSkipped    metric.Int64Counter
	metricsSkipped metric.Int64Counter
	tracesSkipped  metric.Int64Counter
}

func newSchemaProcessor(_ context.Context, conf component.Config, set processor.Settings) (*schemaProcessor, error) {
	cfg, ok := conf.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration provided")
	}

	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	m, err := translation.NewManager(
		cfg.Targets,
		set.Logger.Named("schema-manager"),
		translation.ManagerTelemetry{
			CacheHit:  tb.ProcessorSchemaCacheHits,
			CacheMiss: tb.ProcessorSchemaCacheMisses,
		},
	)
	if err != nil {
		return nil, err
	}
	return &schemaProcessor{
		config:         cfg,
		telemetry:      set.TelemetrySettings,
		log:            set.Logger,
		manager:        m,
		logsSkipped:    tb.ProcessorSchemaLogsSkipped,
		metricsSkipped: tb.ProcessorSchemaMetricsSkipped,
		tracesSkipped:  tb.ProcessorSchemaTracesSkipped,
	}, nil
}

func (t schemaProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	var skipped int64
	for rt := 0; rt < ld.ResourceLogs().Len(); rt++ {
		rLogs := ld.ResourceLogs().At(rt)
		resourceSchemaURL := rLogs.SchemaUrl()
		if resourceSchemaURL != "" {
			t.log.Debug("requesting translation for resourceSchemaURL", zap.String("resourceSchemaURL", resourceSchemaURL))
			tr, err := t.manager.
				RequestTranslation(ctx, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				return ld, err
			}
			err = tr.ApplyAllResourceChanges(rLogs, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to apply resource changes", zap.Error(err))
				return ld, err
			}
		}
		for ss := 0; ss < rLogs.ScopeLogs().Len(); ss++ {
			logs := rLogs.ScopeLogs().At(ss)
			schemaURL := cmp.Or(logs.SchemaUrl(), resourceSchemaURL)
			if schemaURL == "" {
				skipped++
				continue
			}
			tr, err := t.manager.
				RequestTranslation(ctx, schemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				continue
			}
			err = tr.ApplyScopeLogChanges(logs, schemaURL)
			if err != nil {
				t.log.Error("failed to apply scope log changes", zap.Error(err))
			}
		}
	}
	if skipped > 0 {
		t.logsSkipped.Add(ctx, skipped)
	}
	return ld, nil
}

func (t schemaProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	var skipped int64
	for mt := 0; mt < md.ResourceMetrics().Len(); mt++ {
		rMetric := md.ResourceMetrics().At(mt)
		resourceSchemaURL := rMetric.SchemaUrl()
		if resourceSchemaURL != "" {
			t.log.Debug("requesting translation for resourceSchemaURL", zap.String("resourceSchemaURL", resourceSchemaURL))
			tr, err := t.manager.RequestTranslation(ctx, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				return md, err
			}
			err = tr.ApplyAllResourceChanges(rMetric, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to apply resource changes", zap.Error(err))
				return md, err
			}
		}
		for sm := 0; sm < rMetric.ScopeMetrics().Len(); sm++ {
			metric := rMetric.ScopeMetrics().At(sm)
			schemaURL := cmp.Or(metric.SchemaUrl(), resourceSchemaURL)
			if schemaURL == "" {
				skipped++
				continue
			}
			tr, err := t.manager.
				RequestTranslation(ctx, schemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				return md, err
			}
			err = tr.ApplyScopeMetricChanges(metric, schemaURL)
			if err != nil {
				t.log.Error("failed to apply scope metric changes", zap.Error(err))
				return md, err
			}
		}
	}
	if skipped > 0 {
		t.metricsSkipped.Add(ctx, skipped)
	}
	return md, nil
}

func (t schemaProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	var skipped int64
	for rt := 0; rt < td.ResourceSpans().Len(); rt++ {
		rTrace := td.ResourceSpans().At(rt)
		resourceSchemaURL := rTrace.SchemaUrl()

		if resourceSchemaURL != "" {
			t.log.Debug("requesting translation for resourceSchemaURL", zap.String("resourceSchemaURL", resourceSchemaURL))
			tr, err := t.manager.
				RequestTranslation(ctx, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				return td, err
			}
			err = tr.ApplyAllResourceChanges(rTrace, resourceSchemaURL)
			if err != nil {
				t.log.Error("failed to apply resource changes", zap.Error(err))
				return td, err
			}
		}
		for ss := 0; ss < rTrace.ScopeSpans().Len(); ss++ {
			span := rTrace.ScopeSpans().At(ss)
			schemaURL := cmp.Or(span.SchemaUrl(), resourceSchemaURL)
			if schemaURL == "" {
				skipped++
				continue
			}
			tr, err := t.manager.
				RequestTranslation(ctx, schemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				continue
			}
			err = tr.ApplyScopeSpanChanges(span, schemaURL)
			if err != nil {
				t.log.Error("failed to apply scope span changes", zap.Error(err))
			}
		}
	}
	if skipped > 0 {
		t.tracesSkipped.Add(ctx, skipped)
	}
	return td, nil
}

// start will add HTTP provider to the manager and prefetch schemas
func (t *schemaProcessor) start(ctx context.Context, host component.Host) error {
	client, err := t.config.ToClient(ctx, host.GetExtensions(), t.telemetry)
	if err != nil {
		return err
	}
	t.manager.AddProvider(translation.NewHTTPProvider(client))

	go func(ctx context.Context) {
		for _, schemaURL := range t.config.Prefetch {
			t.log.Info("prefetching schema", zap.String("url", schemaURL))
			_, _ = t.manager.RequestTranslation(ctx, schemaURL)
		}
	}(ctx)

	return nil
}
