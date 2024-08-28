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

type transformer struct {
	telemetry component.TelemetrySettings
	config    *Config

	log *zap.Logger

	manager translation.Manager
}

func newTransformer(_ context.Context, conf component.Config, set processor.Settings) (*transformer, error) {
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

	return &transformer{
		config:    cfg,
		telemetry: set.TelemetrySettings,
		log:       set.Logger,
		manager:   m,
	}, nil
}

func (t transformer) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	for rl := 0; rl < ld.ResourceLogs().Len(); rl++ {
		rLog := ld.ResourceLogs().At(rl)
		resourceSchemaUrl := rLog.SchemaUrl()
		err := t.manager.
			RequestTranslation(ctx, resourceSchemaUrl).
			ApplyAllResourceChanges(ctx, rLog, resourceSchemaUrl)
		if err != nil {
			return plog.Logs{}, err
		}
		for sl := 0; sl < rLog.ScopeLogs().Len(); sl++ {
			log := rLog.ScopeLogs().At(sl)
			logSchemaUrl := log.SchemaUrl()
			if logSchemaUrl == "" {
				logSchemaUrl = resourceSchemaUrl
			}

			err := t.manager.
				RequestTranslation(ctx, logSchemaUrl).
				ApplyScopeLogChanges(ctx, log, logSchemaUrl)
			if err != nil {
				return plog.Logs{}, err
			}
		}
	}
	return ld, nil
}

func (t transformer) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	for rm := 0; rm < md.ResourceMetrics().Len(); rm++ {
		rMetric := md.ResourceMetrics().At(rm)
		resourceSchemaUrl := rMetric.SchemaUrl()
		err := t.manager.
			RequestTranslation(ctx, resourceSchemaUrl).
			ApplyAllResourceChanges(ctx, rMetric, resourceSchemaUrl)
		if err != nil {
			return pmetric.Metrics{}, err
		}
		for sm := 0; sm < rMetric.ScopeMetrics().Len(); sm++ {
			metric := rMetric.ScopeMetrics().At(sm)
			metricSchemaUrl := metric.SchemaUrl()
			if metricSchemaUrl == "" {
				metricSchemaUrl = resourceSchemaUrl
			}
			err := t.manager.
				RequestTranslation(ctx, metricSchemaUrl).
				ApplyScopeMetricChanges(ctx, metric, metricSchemaUrl)
			if err != nil {
				return pmetric.Metrics{}, err
			}
		}
	}
	return md, nil
}

func (t transformer) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	for rt := 0; rt < td.ResourceSpans().Len(); rt++ {
		rTrace := td.ResourceSpans().At(rt)
		// todo(ankit) do i need to check if this is empty?
		resourceSchemaUrl := rTrace.SchemaUrl()
		err := t.manager.
			RequestTranslation(ctx, resourceSchemaUrl).
			ApplyAllResourceChanges(ctx, rTrace, resourceSchemaUrl)
		if err != nil {
			return ptrace.Traces{}, err
		}
		for ss := 0; ss < rTrace.ScopeSpans().Len(); ss++ {
			span := rTrace.ScopeSpans().At(ss)
			spanSchemaUrl := span.SchemaUrl()
			if spanSchemaUrl == "" {
				spanSchemaUrl = resourceSchemaUrl
			}
			err := t.manager.
				RequestTranslation(ctx, spanSchemaUrl).
				ApplyScopeSpanChanges(ctx, span, spanSchemaUrl)
			if err != nil {
				return ptrace.Traces{}, err
			}
		}
	}
	return td, nil
}

// start will load the remote file definition if it isn't already cached
// and resolve the schema translation file
func (t *transformer) start(ctx context.Context, host component.Host) error {
	var providers []translation.Provider
	// Check for additional extensions that can be checked first before
	// perfomring the http request
	// TODO(MovieStoreGuy): Check for storage extensions

	client, err := t.config.ToClient(ctx, host, t.telemetry)
	if err != nil {
		return err
	}

	if err := t.manager.SetProviders(append(providers, translation.NewHTTPProvider(client))...); err != nil {
		return err
	}
	go func(ctx context.Context) {
		for _, schemaURL := range t.config.Prefetch {
			_ = t.manager.RequestTranslation(ctx, schemaURL)
		}
	}(ctx)

	return nil
}
