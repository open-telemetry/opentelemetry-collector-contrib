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
	"strings"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

var _ component.TracesExporter = (*traceExporterImp)(nil)

const (
	defaultPort = "55680"
)

var (
	errNoResolver                = errors.New("no resolvers specified for the exporter")
	errMultipleResolversProvided = errors.New("only one resolver should be specified")
	errNoTracesInBatch           = errors.New("no traces were found in the batch")
)

type traceExporterImp struct {
	logger *zap.Logger
	config Config
	host   component.Host

	res  resolver
	ring *hashRing

	exporters            map[string]component.TracesExporter
	exporterFactory      component.ExporterFactory
	templateCreateParams component.ExporterCreateParams

	stopped    bool
	shutdownWg sync.WaitGroup
	updateLock sync.RWMutex
}

// Crete new exporter
func newTracesExporter(params component.ExporterCreateParams, cfg configmodels.Exporter) (*traceExporterImp, error) {
	oCfg := cfg.(*Config)

	tmplParams := component.ExporterCreateParams{
		Logger:               params.Logger,
		ApplicationStartInfo: params.ApplicationStartInfo,
	}

	if oCfg.Resolver.DNS != nil && oCfg.Resolver.Static != nil {
		return nil, errMultipleResolversProvided
	}

	var res resolver
	if oCfg.Resolver.Static != nil {
		var err error
		res, err = newStaticResolver(oCfg.Resolver.Static.Hostnames)
		if err != nil {
			return nil, err
		}
	}
	if oCfg.Resolver.DNS != nil {
		dnsLogger := params.Logger.With(zap.String("resolver", "dns"))

		var err error
		res, err = newDNSResolver(dnsLogger, oCfg.Resolver.DNS.Hostname, oCfg.Resolver.DNS.Port)
		if err != nil {
			return nil, err
		}
	}

	if res == nil {
		return nil, errNoResolver
	}

	return &traceExporterImp{
		logger: params.Logger,
		config: *oCfg,

		res: res,

		exporters:            map[string]component.TracesExporter{},
		exporterFactory:      otlpexporter.NewFactory(),
		templateCreateParams: tmplParams,
	}, nil
}

func (e *traceExporterImp) Start(ctx context.Context, host component.Host) error {
	e.res.onChange(e.onBackendChanges)
	e.host = host
	if err := e.res.start(ctx); err != nil {
		return err
	}

	return nil
}

func (e *traceExporterImp) onBackendChanges(resolved []string) {
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

func (e *traceExporterImp) addMissingExporters(ctx context.Context, endpoints []string) {
	for _, endpoint := range endpoints {
		endpoint = endpointWithPort(endpoint)

		if _, exists := e.exporters[endpoint]; !exists {
			cfg := e.buildExporterConfig(endpoint)
			exp, err := e.exporterFactory.CreateTracesExporter(ctx, e.templateCreateParams, &cfg)
			if err != nil {
				e.logger.Error("failed to create new trace exporter for endpoint", zap.String("endpoint", endpoint), zap.Error(err))
				continue
			}
			if err = exp.Start(ctx, e.host); err != nil {
				e.logger.Error("failed to start new trace exporter for endpoint", zap.String("endpoint", endpoint), zap.Error(err))
				continue
			}
			e.exporters[endpoint] = exp
		}
	}
}

func (e *traceExporterImp) buildExporterConfig(endpoint string) otlpexporter.Config {
	oCfg := e.config.Protocol.OTLP
	oCfg.Endpoint = endpoint
	return oCfg
}

func (e *traceExporterImp) removeExtraExporters(ctx context.Context, endpoints []string) {
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

func (e *traceExporterImp) Shutdown(context.Context) error {
	e.stopped = true
	e.shutdownWg.Wait()
	return nil
}

func (e *traceExporterImp) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	var errors []error
	batches := batchpersignal.SplitTraces(td)
	for _, batch := range batches {
		if err := e.consumeTrace(ctx, batch); err != nil {
			errors = append(errors, err)
		}
	}

	return consumererror.CombineErrors(errors)
}

func (e *traceExporterImp) consumeTrace(ctx context.Context, td pdata.Traces) error {
	traceID := traceIDFromTraces(td)
	if traceID == pdata.InvalidTraceID() {
		return errNoTracesInBatch
	}

	// NOTE: make rolling updates of next tier of collectors work. currently this may cause
	// data loss because the latest batches sent to outdated backend will never find their way out.
	// for details: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	e.updateLock.RLock()
	endpoint := e.ring.endpointFor(traceID)
	exp, found := e.exporters[endpoint]
	e.updateLock.RUnlock()
	if !found {
		// something is really wrong... how come we couldn't find the exporter??
		return fmt.Errorf("couldn't find the exporter for the endpoint %q", endpoint)
	}

	start := time.Now()
	err := exp.ConsumeTraces(ctx, td)
	duration := time.Since(start)
	ctx, _ = tag.New(ctx, tag.Upsert(tag.MustNewKey("endpoint"), endpoint))

	if err == nil {
		sCtx, _ := tag.New(ctx, tag.Upsert(tag.MustNewKey("success"), "true"))
		stats.Record(sCtx, mBackendLatency.M(duration.Milliseconds()))
	} else {
		fCtx, _ := tag.New(ctx, tag.Upsert(tag.MustNewKey("success"), "false"))
		stats.Record(fCtx, mBackendLatency.M(duration.Milliseconds()))
	}

	return err
}

func (e *traceExporterImp) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: false}
}

func traceIDFromTraces(td pdata.Traces) pdata.TraceID {
	rs := td.ResourceSpans()
	if rs.Len() == 0 {
		return pdata.InvalidTraceID()
	}

	ils := rs.At(0).InstrumentationLibrarySpans()
	if ils.Len() == 0 {
		return pdata.InvalidTraceID()
	}

	spans := ils.At(0).Spans()
	if spans.Len() == 0 {
		return pdata.InvalidTraceID()
	}

	return spans.At(0).TraceID()
}

func endpointWithPort(endpoint string) string {
	if !strings.Contains(endpoint, ":") {
		endpoint = fmt.Sprintf("%s:%s", endpoint, defaultPort)
	}
	return endpoint
}
