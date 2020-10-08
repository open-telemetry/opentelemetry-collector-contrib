// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.uber.org/zap"
)

var _ component.TraceExporter = (*exporterImp)(nil)

const (
	defaultResInterval    = 5 * time.Second
	defaultResTimeout     = time.Second
	defaultEndpointFormat = "%s:55678"
)

var errNoResolver = errors.New("no resolvers specified for the exporter")

type exporterImp struct {
	logger *zap.Logger
	config Config

	res  resolver
	ring *hashRing

	exporters            map[string]component.TraceExporter
	exporterFactory      component.ExporterFactory
	templateCreateParams component.ExporterCreateParams

	stopped    bool
	shutdownWg sync.WaitGroup
	updateLock sync.RWMutex
}

// Crete new exporter
func newExporter(params component.ExporterCreateParams, cfg configmodels.Exporter) (*exporterImp, error) {
	oCfg := cfg.(*Config)

	tmplParams := component.ExporterCreateParams{
		Logger:               params.Logger,
		ApplicationStartInfo: params.ApplicationStartInfo,
	}

	var res resolver
	if oCfg.Resolver.Static != nil {
		var err error
		res, err = newStaticResolver(oCfg.Resolver.Static.Hostnames)
		if err != nil {
			return nil, err
		}
	}

	if res == nil {
		return nil, errNoResolver
	}

	return &exporterImp{
		logger: params.Logger,
		config: *oCfg,

		res: res,

		exporters:            map[string]component.TraceExporter{},
		exporterFactory:      otlpexporter.NewFactory(),
		templateCreateParams: tmplParams,
	}, nil
}

func (e *exporterImp) Start(ctx context.Context, host component.Host) error {
	e.res.onChange(e.onBackendChanges)
	if err := e.res.start(ctx); err != nil {
		return err
	}

	return nil
}

func (e *exporterImp) onBackendChanges(resolved []string) {
	resolved = sort.StringSlice(resolved)
	newRing := newHashRing(resolved)

	if !newRing.equal(e.ring) {
		e.updateLock.Lock()
		defer e.updateLock.Unlock()

		e.ring = newRing

		// TODO: set a timeout?
		ctx := context.Background()

		// add the missing exporters first
		e.addMissingExporters(ctx, resolved)
		e.removeExtraExporters(ctx, resolved)
	}
}

func (e *exporterImp) addMissingExporters(ctx context.Context, endpoints []string) {
	for _, endpoint := range endpoints {
		if _, exists := e.exporters[endpoint]; !exists {
			cfg := e.exporterFactory.CreateDefaultConfig()
			oCfg := cfg.(*otlpexporter.Config)

			// is this wrong? are we overriding default fields, or is the config unmarshaller taking care of it?
			oCfg.ExporterSettings = e.config.OTLP.ExporterSettings
			oCfg.TimeoutSettings = e.config.OTLP.TimeoutSettings
			oCfg.QueueSettings = e.config.OTLP.QueueSettings
			oCfg.RetrySettings = e.config.OTLP.RetrySettings
			oCfg.GRPCClientSettings = e.config.OTLP.GRPCClientSettings

			oCfg.Endpoint = fmt.Sprintf(defaultEndpointFormat, endpoint)
			exp, err := e.exporterFactory.CreateTraceExporter(ctx, e.templateCreateParams, cfg)
			if err != nil {
				e.logger.Warn("failed to create new trace exporter for endpoint", zap.String("endpoint", endpoint))
				continue
			}
			e.exporters[endpoint] = exp
		}
	}
}

func (e *exporterImp) removeExtraExporters(ctx context.Context, endpoints []string) {
	for existing := range e.exporters {
		if !endpointFound(existing, endpoints) {
			e.exporters[existing].Shutdown(ctx)
			delete(e.exporters, existing)
		}
	}
}

func endpointFound(endpoint string, endpoints []string) bool {
	for _, candidate := range endpoints {
		if candidate == endpoint {
			return true
		}
	}

	return false
}

func (e *exporterImp) Shutdown(context.Context) error {
	e.stopped = true
	e.shutdownWg.Wait()
	return nil
}

func (e *exporterImp) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	e.updateLock.RLock()
	defer e.updateLock.RUnlock()

	traceID := traceIDFromTraces(td)
	endpoint := e.ring.endpointFor(traceID)
	exp, found := e.exporters[endpoint]
	if !found {
		// something is really wrong... how come we couldn't find the exporter??
		return fmt.Errorf("couldn't find the exporter for the endpoint %q", endpoint)
	}
	return exp.ConsumeTraces(ctx, td)
}

func (e *exporterImp) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: false}
}

func traceIDFromTraces(td pdata.Traces) pdata.TraceID {
	// is this safe? can a trace be empty and not contain a single span?
	return td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).TraceID()
}
