// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)

type transformer struct {
	telemetry component.TelemetrySettings
	config    *Config

	log *zap.Logger

	manager translation.Manager
}

func newTransformer(ctx context.Context, conf config.Processor, set component.ProcessorCreateSettings) (*transformer, error) {
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
		t.manager.
			RequestTranslation(ctx, rLog.SchemaUrl()).
			ApplyAllResourceChanges(ctx, rLog)
		for sl := 0; sl < rLog.ScopeLogs().Len(); sl++ {
			log := rLog.ScopeLogs().At(sl)
			t.manager.
				RequestTranslation(ctx, log.SchemaUrl()).
				ApplyScopeLogChanges(ctx, log)
		}
	}
	return ld, nil
}

func (t transformer) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	for rm := 0; rm < md.ResourceMetrics().Len(); rm++ {
		rMetric := md.ResourceMetrics().At(rm)
		t.manager.
			RequestTranslation(ctx, rMetric.SchemaUrl()).
			ApplyAllResourceChanges(ctx, rMetric)
		for sm := 0; sm < rMetric.ScopeMetrics().Len(); sm++ {
			metric := rMetric.ScopeMetrics().At(sm)
			t.manager.
				RequestTranslation(ctx, metric.SchemaUrl()).
				ApplyScopeMetricChanges(ctx, metric)
		}
	}
	return md, nil
}

func (t transformer) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	for rt := 0; rt < td.ResourceSpans().Len(); rt++ {
		rTrace := td.ResourceSpans().At(rt)
		t.manager.
			RequestTranslation(ctx, rTrace.SchemaUrl()).
			ApplyAllResourceChanges(ctx, rTrace)
		for ss := 0; ss < rTrace.ScopeSpans().Len(); ss++ {
			span := rTrace.ScopeSpans().At(ss)
			t.manager.
				RequestTranslation(ctx, span.SchemaUrl()).
				ApplyScopeSpanChanges(ctx, span)
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

	client, err := t.config.ToClient(host.GetExtensions(), t.telemetry)
	if err != nil {
		return err
	}

	t.manager.SetProviders(append(providers, translation.NewHTTPProvider(client))...)
	go func(ctx context.Context) {
		for _, schemaURL := range t.config.Prefetch {
			_ = t.manager.RequestTranslation(ctx, schemaURL)
		}
	}(ctx)

	return nil
}
