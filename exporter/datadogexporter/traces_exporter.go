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

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/agent"
	traceconfig "github.com/DataDog/datadog-agent/pkg/trace/config"
	tracelog "github.com/DataDog/datadog-agent/pkg/trace/log"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/service/featuregate"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/source"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils"

	modelsource "github.com/DataDog/datadog-agent/pkg/otlp/model/source"
)

type traceExporter struct {
	params         component.ExporterCreateSettings
	cfg            *Config
	ctx            context.Context // ctx triggers shutdown upon cancellation
	client         *datadog.Client // client sends runnimg metrics to backend & performs API validation
	scrubber       scrub.Scrubber  // scrubber scrubs sensitive information from error messages
	onceMetadata   *sync.Once      // onceMetadata ensures that metadata is sent only once across all exporters
	wg             sync.WaitGroup  // wg waits for graceful shutdown
	agent          *agent.Agent    // agent processes incoming traces
	sourceProvider source.Provider // is able to source the origin of a trace (hostname, container, etc)
}

func newTracesExporter(ctx context.Context, params component.ExporterCreateSettings, cfg *Config, onceMetadata *sync.Once, sourceProvider source.Provider) (*traceExporter, error) {
	// client to send running metric to the backend & perform API key validation
	client := utils.CreateClient(cfg.API.Key, cfg.Metrics.TCPAddr.Endpoint)
	if err := utils.ValidateAPIKey(params.Logger, client); err != nil && cfg.API.FailOnInvalidKey {
		return nil, err
	}
	acfg := traceconfig.New()
	acfg.AgentVersion = fmt.Sprintf("datadogexporter-%s-%s", params.BuildInfo.Command, params.BuildInfo.Version)
	src, err := sourceProvider.Source(ctx)
	if err != nil {
		return nil, err
	}
	if src.Kind == source.HostnameKind {
		acfg.Hostname = src.Identifier
	}
	acfg.OTLPReceiver.SpanNameRemappings = cfg.Traces.SpanNameRemappings
	acfg.OTLPReceiver.SpanNameAsResourceName = cfg.Traces.SpanNameAsResourceName
	acfg.OTLPReceiver.UsePreviewHostnameLogic = featuregate.GetRegistry().IsEnabled(metadata.HostnamePreviewFeatureGate)
	acfg.Endpoints[0].APIKey = cfg.API.Key
	acfg.Ignore["resource"] = cfg.Traces.IgnoreResources
	acfg.ReceiverPort = 0 // disable HTTP receiver
	if v := cfg.Traces.flushInterval; v > 0 {
		acfg.TraceWriter.FlushPeriodSeconds = v
	}
	if addr := cfg.Traces.Endpoint; addr != "" {
		acfg.Endpoints[0].Host = addr
	}
	tracelog.SetLogger(&zaplogger{params.Logger})
	agnt := agent.NewAgent(ctx, acfg)
	exp := &traceExporter{
		params:         params,
		cfg:            cfg,
		ctx:            ctx,
		client:         client,
		agent:          agnt,
		onceMetadata:   onceMetadata,
		scrubber:       scrub.NewScrubber(),
		sourceProvider: sourceProvider,
	}
	exp.wg.Add(1)
	go func() {
		defer exp.wg.Done()
		agnt.Run()
	}()
	return exp, nil
}

var _ consumer.ConsumeTracesFunc = (*traceExporter)(nil).consumeTraces

func (exp *traceExporter) consumeTraces(
	ctx context.Context,
	td ptrace.Traces,
) (err error) {
	defer func() { err = exp.scrubber.Scrub(err) }()
	if exp.cfg.HostMetadata.Enabled {
		// start host metadata with resource attributes from
		// the first payload.
		exp.onceMetadata.Do(func() {
			attrs := pcommon.NewMap()
			if td.ResourceSpans().Len() > 0 {
				attrs = td.ResourceSpans().At(0).Resource().Attributes()
			}
			go metadata.Pusher(exp.ctx, exp.params, newMetadataConfigfromConfig(exp.cfg), exp.sourceProvider, attrs)
		})
	}
	rspans := td.ResourceSpans()
	hosts := make(map[string]struct{})
	tags := make(map[string]struct{})
	now := pcommon.NewTimestampFromTime(time.Now())
	for i := 0; i < rspans.Len(); i++ {
		rspan := rspans.At(i)
		src := exp.agent.OTLPReceiver.ReceiveResourceSpans(rspan, http.Header{}, "otlp-exporter")
		switch src.Kind {
		case modelsource.HostnameKind:
			hosts[src.Identifier] = struct{}{}
		case modelsource.AWSECSFargateKind:
			tags[src.Tag()] = struct{}{}
		}
	}
	series := make([]datadog.Metric, 0, len(hosts)+len(tags))
	for host := range hosts {
		series = append(series, metrics.DefaultMetrics("traces", host, uint64(now), exp.params.BuildInfo)...)
	}
	for tag := range tags {
		ms := metrics.DefaultMetrics("traces", "", uint64(now), exp.params.BuildInfo)
		for i := range ms {
			ms[i].Tags = append(ms[i].Tags, tag)
		}
		series = append(series, ms...)
	}
	if err := exp.client.PostMetrics(series); err != nil {
		exp.params.Logger.Error("Error posting hostname/tags series", zap.Error(err))
	}
	return nil
}

func (exp *traceExporter) waitShutdown() {
	exp.wg.Wait()
}
