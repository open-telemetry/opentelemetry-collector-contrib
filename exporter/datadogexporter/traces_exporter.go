// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// nolint:errcheck
package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/config/configdefs"
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils"
)

type traceExporter struct {
	params         component.ExporterCreateSettings
	cfg            *config.Config
	ctx            context.Context
	edgeConnection traceEdgeConnection
	obfuscator     *obfuscate.Obfuscator
	client         *datadog.Client
	denylister     *denylister
	scrubber       scrub.Scrubber
	onceMetadata   *sync.Once
}

var (
	obfuscatorConfig = &configdefs.ObfuscationConfig{
		ES: configdefs.JSONObfuscationConfig{
			Enabled: true,
		},
		Mongo: configdefs.JSONObfuscationConfig{
			Enabled: true,
		},
		HTTP: configdefs.HTTPObfuscationConfig{
			RemoveQueryString: true,
			RemovePathDigits:  true,
		},
		RemoveStackTraces: true,
		Redis:             configdefs.Enablable{Enabled: true},
		Memcached:         configdefs.Enablable{Enabled: true},
	}
)

func newTracesExporter(ctx context.Context, params component.ExporterCreateSettings, cfg *config.Config, onceMetadata *sync.Once) (*traceExporter, error) {
	// client to send running metric to the backend & perform API key validation
	client := utils.CreateClient(cfg.API.Key, cfg.Metrics.TCPAddr.Endpoint)
	if err := utils.ValidateAPIKey(params.Logger, client); err != nil && cfg.API.FailOnInvalidKey {
		return nil, err
	}

	// removes potentially sensitive info and PII, approach taken from serverless approach
	// https://github.com/DataDog/datadog-serverless-functions/blob/11f170eac105d66be30f18eda09eca791bc0d31b/aws/logs_monitoring/trace_forwarder/cmd/trace/main.go#L43
	obfuscator := obfuscate.NewObfuscator(obfuscatorConfig)

	// a denylist for dropping ignored resources
	denylister := newDenylister(cfg.Traces.IgnoreResources)

	exporter := &traceExporter{
		params:         params,
		cfg:            cfg,
		ctx:            ctx,
		edgeConnection: createTraceEdgeConnection(cfg.Traces.TCPAddr.Endpoint, cfg.API.Key, params.BuildInfo, cfg.TimeoutSettings, cfg.LimitedHTTPClientSettings),
		obfuscator:     obfuscator,
		client:         client,
		denylister:     denylister,
		scrubber:       scrub.NewScrubber(),
		onceMetadata:   onceMetadata,
	}

	return exporter, nil
}

// TODO: when component.Host exposes a way to retrieve processors, check for batch processors
// and log a warning if not set

// Start tells the exporter to start. The exporter may prepare for exporting
// by connecting to the endpoint. Host parameter can be used for communicating
// with the host after Start() has already returned. If error is returned by
// Start() then the collector startup will be aborted.
// func (exp *traceExporter) Start(_ context.Context, _ component.Host) error {
// 	return nil
// }

func (exp *traceExporter) pushTraceDataScrubbed(ctx context.Context, td ptrace.Traces) error {
	return exp.scrubber.Scrub(exp.pushTraceData(ctx, td))
}

// force pushTraceData to be a ConsumeTracesFunc, even if no error is returned.
var _ consumer.ConsumeTracesFunc = (*traceExporter)(nil).pushTraceData

func (exp *traceExporter) pushTraceData(
	ctx context.Context,
	td ptrace.Traces,
) error {

	// Start host metadata with resource attributes from
	// the first payload.
	if exp.cfg.HostMetadata.Enabled {
		exp.onceMetadata.Do(func() {
			attrs := pcommon.NewMap()
			if td.ResourceSpans().Len() > 0 {
				attrs = td.ResourceSpans().At(0).Resource().Attributes()
			}
			go metadata.Pusher(exp.ctx, exp.params, newMetadataConfigfromConfig(exp.cfg), attrs)
		})
	}

	// convert traces to datadog traces and group trace payloads by env
	// we largely apply the same logic as the serverless implementation, simplified a bit
	// https://github.com/DataDog/datadog-serverless-functions/blob/f5c3aedfec5ba223b11b76a4239fcbf35ec7d045/aws/logs_monitoring/trace_forwarder/cmd/trace/main.go#L61-L83
	fallbackHost := metadata.GetHost(exp.params.Logger, exp.cfg.Hostname)
	ddTraces, ms := convertToDatadogTd(td, fallbackHost, exp.cfg, exp.denylister, exp.params.BuildInfo)

	// group the traces by env to reduce the number of flushes
	aggregatedTraces := aggregateTracePayloadsByEnv(ddTraces)

	// security/obfuscation for db, query strings, stack traces, pii, etc
	// TODO: is there any config we want here? OTEL has their own pipeline for regex obfuscation
	obfuscatePayload(exp.obfuscator, aggregatedTraces)

	pushTime := time.Now().UTC().UnixNano()
	for _, ddTracePayload := range aggregatedTraces {
		// currently we don't want to do retries since api endpoints may not dedupe in certain situations
		// adding a helper function here to make custom retry logic easier in the future
		exp.pushWithRetry(ctx, ddTracePayload, 1, pushTime, func() error {
			return nil
		})
	}

	_ = exp.client.PostMetrics(ms)

	return nil
}

// gives us flexibility to add custom retry logic later
func (exp *traceExporter) pushWithRetry(ctx context.Context, ddTracePayload *pb.TracePayload, maxRetries int, pushTime int64, fn func() error) error {
	err := exp.edgeConnection.SendTraces(ctx, ddTracePayload, maxRetries)

	if err != nil {
		exp.params.Logger.Info("failed to send traces", zap.Error(err))
	}

	// this is for generating metrics like hits, errors, and latency, it uses a separate endpoint than Traces
	stats := computeAPMStats(ddTracePayload, pushTime)
	errStats := exp.edgeConnection.SendStats(context.Background(), stats, maxRetries)

	if errStats != nil {
		exp.params.Logger.Info("failed to send trace stats", zap.Error(errStats))
	}

	return fn()
}
