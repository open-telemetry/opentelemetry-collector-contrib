// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)

type schemaProcessor struct {
	telemetry component.TelemetrySettings
	config    *Config

	log *zap.Logger

	manager translation.Manager
}

func newSchemaProcessor(_ context.Context, conf component.Config, set processor.Settings) (*schemaProcessor, error) {
	cfg, ok := conf.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration provided")
	}

	m, err := translation.NewManager(
		cfg.Targets,
		set.Logger.Named("schema-manager"),
	)
	if err != nil {
		return nil, err
	}
	return &schemaProcessor{
		config:    cfg,
		telemetry: set.TelemetrySettings,
		log:       set.Logger,
		manager:   m,
	}, nil
}

func (t schemaProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
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
			logsSchemaURL := logs.SchemaUrl()
			if logsSchemaURL == "" {
				logsSchemaURL = resourceSchemaURL
			}
			if logsSchemaURL == "" {
				continue
			}
			tr, err := t.manager.
				RequestTranslation(ctx, logsSchemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				continue
			}
			err = tr.ApplyScopeLogChanges(logs, logsSchemaURL)
			if err != nil {
				t.log.Error("failed to apply scope log changes", zap.Error(err))
			}
		}
	}
	return ld, nil
}

func (t schemaProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	return md, nil
}

func (t schemaProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
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
			spanSchemaURL := span.SchemaUrl()
			if spanSchemaURL == "" {
				spanSchemaURL = resourceSchemaURL
			}
			if spanSchemaURL == "" {
				continue
			}
			tr, err := t.manager.
				RequestTranslation(ctx, spanSchemaURL)
			if err != nil {
				t.log.Error("failed to request translation", zap.Error(err))
				continue
			}
			err = tr.ApplyScopeSpanChanges(span, spanSchemaURL)
			if err != nil {
				t.log.Error("failed to apply scope span changes", zap.Error(err))
			}
		}
	}
	return td, nil
}

// start will add HTTP provider to the manager and prefetch schemas
func (t *schemaProcessor) start(ctx context.Context, host component.Host) error {
	client, err := t.config.ToClient(ctx, host, t.telemetry)
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
