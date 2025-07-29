// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	gzip "github.com/DataDog/datadog-agent/comp/trace/compression/impl-gzip"
	"github.com/DataDog/datadog-agent/pkg/trace/agent"
	traceconfig "github.com/DataDog/datadog-agent/pkg/trace/config"
	tracelog "github.com/DataDog/datadog-agent/pkg/trace/log"
	"github.com/DataDog/datadog-agent/pkg/trace/telemetry"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	zorkian "gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/scrub"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var traceCustomHTTPFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.datadogexporter.TraceExportUseCustomHTTPClient",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, trace export uses the HTTP client from the exporter HTTP configs"),
	featuregate.WithRegisterFromVersion("v0.105.0"),
)

type traceExporter struct {
	params           exporter.Settings
	cfg              *datadogconfig.Config
	ctx              context.Context          // ctx triggers shutdown upon cancellation
	client           *zorkian.Client          // client sends running metrics to backend & performs API validation
	metricsAPI       *datadogV2.MetricsApi    // client sends running metrics to backend
	scrubber         scrub.Scrubber           // scrubber scrubs sensitive information from error messages
	onceMetadata     *sync.Once               // onceMetadata ensures that metadata is sent only once across all exporters
	agent            *agent.Agent             // agent processes incoming traces
	sourceProvider   source.Provider          // is able to source the origin of a trace (hostname, container, etc)
	metadataReporter *inframetadata.Reporter  // reports host metadata from resource attributes and metrics
	retrier          *clientutil.Retrier      // retrier handles retries on requests
	gatewayUsage     *attributes.GatewayUsage // gatewayUsage stores the gateway usage metrics
}

func newTracesExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg *datadogconfig.Config,
	onceMetadata *sync.Once,
	sourceProvider source.Provider,
	agent *agent.Agent,
	metadataReporter *inframetadata.Reporter,
	gatewayUsage *attributes.GatewayUsage,
) (*traceExporter, error) {
	scrubber := scrub.NewScrubber()
	exp := &traceExporter{
		params:           params,
		cfg:              cfg,
		ctx:              ctx,
		agent:            agent,
		onceMetadata:     onceMetadata,
		scrubber:         scrubber,
		sourceProvider:   sourceProvider,
		retrier:          clientutil.NewRetrier(params.Logger, cfg.BackOffConfig, scrubber),
		metadataReporter: metadataReporter,
		gatewayUsage:     gatewayUsage,
	}
	// client to send running metric to the backend & perform API key validation
	errchan := make(chan error)
	if isMetricExportV2Enabled() {
		apiClient := clientutil.CreateAPIClient(
			params.BuildInfo,
			cfg.Metrics.Endpoint,
			cfg.ClientConfig)
		go func() { errchan <- clientutil.ValidateAPIKey(ctx, string(cfg.API.Key), params.Logger, apiClient) }()
		exp.metricsAPI = datadogV2.NewMetricsApi(apiClient)
	} else {
		client := clientutil.CreateZorkianClient(string(cfg.API.Key), cfg.Metrics.Endpoint)
		go func() { errchan <- clientutil.ValidateAPIKeyZorkian(params.Logger, client) }()
		exp.client = client
	}
	if cfg.API.FailOnInvalidKey {
		if err := <-errchan; err != nil {
			return nil, err
		}
	}
	return exp, nil
}

var _ consumer.ConsumeTracesFunc = (*traceExporter)(nil).consumeTraces

// headerComputedStats specifies the HTTP header which indicates whether APM stats
// have already been computed for a payload.
const headerComputedStats = "Datadog-Client-Computed-Stats"

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
			go hostmetadata.RunPusher(exp.ctx, exp.params, newMetadataConfigfromConfig(exp.cfg), exp.sourceProvider, attrs, exp.metadataReporter)
		})

		// Consume resources for host metadata
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			res := td.ResourceSpans().At(i).Resource()
			consumeResource(exp.metadataReporter, res, exp.params.Logger)
		}
	}
	rspans := td.ResourceSpans()
	hosts := make(map[string]struct{})
	tags := make(map[string]struct{})
	header := make(http.Header)
	if noAPMStatsFeatureGate.IsEnabled() {
		header[headerComputedStats] = []string{"true"}
	}
	for i := range rspans.Len() {
		rspan := rspans.At(i)
		sspans := rspan.ScopeSpans()

		for j := range sspans.Len() {
			sspan := sspans.At(j)
			spans := sspan.Spans()

			for k := range spans.Len() {
				span := spans.At(k)

				_, isRum := span.Attributes().Get("session.id")
				if isRum {

					client := &http.Client{
						Timeout: 10 * time.Second,
					}

					rattr := rspan.Resource().Attributes()
					sattr := span.Attributes()

					var rawRumData pcommon.Value
					rawRumData, _ = rattr.Get("request_body_dump")

					// build the Datadog intake URL
					ddforward, _ := rattr.Get("request_ddforward")
					outUrlString := "https://browser-intake-datadoghq.com" +
						ddforward.AsString()

					// forward the request to the Datadog intake URL using the POST method
					req, err := http.NewRequest("POST", outUrlString, bytes.NewBuffer(rawRumData.Bytes().AsRaw()))
					if err != nil {
						exp.params.Logger.Info("failed to create request: %v", zap.Error(err))
					}

					// add X-Forwarded-For header containing the request client IP address
					ip, ok := sattr.Get("client.address")
					if ok {
						req.Header.Add("X-Forwarded-For", ip.AsString())
					}
					req.Header.Set("Content-Type", "application/json")

					// we're currently not setting the request headers in the resource on the receiver side
					/*
						headersMap, _ := rattr.Get("request_headers")
						headersMap.Map().Range(func(key string, v pcommon.Value) bool {
							exp.params.Logger.Debug("setting header:")
							exp.params.Logger.Debug(key)
							for i := range v.Slice().Len() {
								exp.params.Logger.Debug(v.Slice().At(i).AsString())
								req.Header.Set(key, v.Slice().At(i).AsString())
							}
							return true
						})
					*/

					// construct request body according to RUM spec
					rumPayload := constructRumPayloadFromOTLP(sattr)

					byts, err := json.Marshal(rumPayload)
					if err != nil {
						return fmt.Errorf("failed to marshal RUM payload: %v", err)
					}
					req.Body = io.NopCloser(bytes.NewBuffer(byts))
					// send the request to the Datadog intake URL
					resp, err := client.Do(req)
					if err != nil {
						return fmt.Errorf("failed to send request: %v", err)
					}
					defer func(Body io.ReadCloser) {
						err := Body.Close()
						if err != nil {
						}
					}(resp.Body)

					// read the response body
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return fmt.Errorf("failed to read response: %v", err)
					}

					// check the status code of the response
					if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
						return fmt.Errorf("received non-OK response: status: %s, body: %s", resp.Status, body)
					}
					fmt.Println("Response:", string(body))
					continue
				}
			}
			// remove the RUM spans from the resource so that they are not exported directly to the backend
			// this is a temporary fix to avoid duplicates, but eventually we should make the necessary changes to
			// ReceiveResourceSpans in agent to handle the exclusion of RUM events from the resource spans
			sspan.Spans().RemoveIf(func(span ptrace.Span) bool {
				_, isRum := span.Attributes().Get("session.id")
				return isRum
			})
		}
		src := exp.agent.OTLPReceiver.ReceiveResourceSpans(ctx, rspan, header, exp.gatewayUsage)
		switch src.Kind {
		case source.HostnameKind:
			hosts[src.Identifier] = struct{}{}
		case source.AWSECSFargateKind:
			tags[src.Tag()] = struct{}{}
		case source.InvalidKind:
		}
	}

	exp.exportUsageMetrics(ctx, hosts, tags)
	return nil
}

func (exp *traceExporter) exportUsageMetrics(ctx context.Context, hosts, tags map[string]struct{}) {
	now := pcommon.NewTimestampFromTime(time.Now())
	buildTags := metrics.TagsFromBuildInfo(exp.params.BuildInfo)
	var err error
	if isMetricExportV2Enabled() {
		series := make([]datadogV2.MetricSeries, 0, len(hosts)+len(tags))
		timestamp := uint64(now)
		for host := range hosts {
			series = append(series, metrics.DefaultMetrics("traces", host, timestamp, buildTags)...)
			if exp.gatewayUsage != nil {
				series = append(series, metrics.GatewayUsageGauge(timestamp, host, buildTags, exp.gatewayUsage))
			}
		}
		for tag := range tags {
			ms := metrics.DefaultMetrics("traces", "", timestamp, buildTags)
			for i := range ms {
				ms[i].Tags = append(ms[i].Tags, tag)
			}
			series = append(series, ms...)
		}
		_, err = exp.retrier.DoWithRetries(ctx, func(context.Context) error {
			ctx2 := clientutil.GetRequestContext(ctx, string(exp.cfg.API.Key))
			_, httpresp, merr := exp.metricsAPI.SubmitMetrics(ctx2, datadogV2.MetricPayload{Series: series}, *clientutil.GZipSubmitMetricsOptionalParameters)
			return clientutil.WrapError(merr, httpresp)
		})
	} else {
		series := make([]zorkian.Metric, 0, len(hosts)+len(tags))
		for host := range hosts {
			series = append(series, metrics.DefaultZorkianMetrics("traces", host, uint64(now), exp.params.BuildInfo)...)
		}
		for tag := range tags {
			ms := metrics.DefaultZorkianMetrics("traces", "", uint64(now), exp.params.BuildInfo)
			for i := range ms {
				ms[i].Tags = append(ms[i].Tags, tag)
			}
			series = append(series, ms...)
		}
		_, err = exp.retrier.DoWithRetries(ctx, func(context.Context) error {
			return exp.client.PostMetrics(series)
		})
	}
	if err != nil {
		exp.params.Logger.Error("Error posting hostname/tags series", zap.Error(err))
	}
}

func newTraceAgent(ctx context.Context, params exporter.Settings, cfg *datadogconfig.Config, sourceProvider source.Provider, metricsClient statsd.ClientInterface, attrsTranslator *attributes.Translator) (*agent.Agent, error) {
	acfg, err := newTraceAgentConfig(ctx, params, cfg, sourceProvider, attrsTranslator)
	if err != nil {
		return nil, err
	}
	return agent.NewAgent(ctx, acfg, telemetry.NewNoopCollector(), metricsClient, gzip.NewComponent()), nil
}

func newTraceAgentConfig(ctx context.Context, params exporter.Settings, cfg *datadogconfig.Config, sourceProvider source.Provider, attrsTranslator *attributes.Translator) (*traceconfig.AgentConfig, error) {
	acfg := traceconfig.New()
	src, err := sourceProvider.Source(ctx)
	if err != nil {
		return nil, err
	}
	if src.Kind == source.HostnameKind {
		acfg.Hostname = src.Identifier
	}
	acfg.OTLPReceiver.AttributesTranslator = attrsTranslator
	acfg.OTLPReceiver.SpanNameRemappings = cfg.Traces.SpanNameRemappings
	acfg.OTLPReceiver.SpanNameAsResourceName = cfg.Traces.SpanNameAsResourceName
	acfg.Endpoints[0].APIKey = string(cfg.API.Key)
	acfg.Ignore["resource"] = cfg.Traces.IgnoreResources
	acfg.ReceiverEnabled = false // disable HTTP receiver
	acfg.AgentVersion = fmt.Sprintf("datadogexporter-%s-%s", params.BuildInfo.Command, params.BuildInfo.Version)
	acfg.SkipSSLValidation = cfg.TLS.InsecureSkipVerify
	acfg.ComputeStatsBySpanKind = cfg.Traces.ComputeStatsBySpanKind
	acfg.PeerTagsAggregation = cfg.Traces.PeerTagsAggregation
	acfg.PeerTags = cfg.Traces.PeerTags
	acfg.MaxSenderRetries = 4
	if traceCustomHTTPFeatureGate.IsEnabled() {
		params.Logger.Info("Experimental feature: datadog exporter trace export uses a custom HTTP client from the exporter HTTP configs")
		acfg.HTTPClientFunc = func() *http.Client {
			return clientutil.NewHTTPClient(cfg.ClientConfig)
		}
	}
	if !datadog.OperationAndResourceNameV2FeatureGate.IsEnabled() {
		acfg.Features["disable_operation_and_resource_name_logic_v2"] = struct{}{}
	}
	if v := cfg.Traces.GetFlushInterval(); v > 0 {
		acfg.TraceWriter.FlushPeriodSeconds = v
	}
	if v := cfg.Traces.TraceBuffer; v > 0 {
		acfg.TraceBuffer = v
	}
	if addr := cfg.Traces.Endpoint; addr != "" {
		acfg.Endpoints[0].Host = addr
	}
	if cfg.Traces.ComputeTopLevelBySpanKind {
		acfg.Features["enable_otlp_compute_top_level_by_span_kind"] = struct{}{}
	}
	if !datadog.ReceiveResourceSpansV2FeatureGate.IsEnabled() {
		acfg.Features["disable_receive_resource_spans_v2"] = struct{}{}
	}
	tracelog.SetLogger(&agentcomponents.ZapLogger{Logger: params.Logger}) // TODO: This shouldn't be a singleton
	return acfg, nil
}

func buildRumPayload(k string, v pcommon.Value, rumPayload map[string]any) {
	parts := strings.Split(k, ".")

	current := rumPayload
	for i, part := range parts {
		if i == len(parts)-1 {
			if v.Type() == pcommon.ValueTypeSlice {
				current[part] = v.Slice().AsRaw()
			} else if v.Type() == pcommon.ValueTypeMap {
				current[part] = v.Map().AsRaw()
			} else {
				current[part] = v.AsRaw()
			}
		} else {
			if _, ok := current[part]; !ok {
				current[part] = make(map[string]any)
			}

			// in case the current part is not a map, we should override it with a map to avoid type assertion errors
			next, ok := current[part].(map[string]any)
			if !ok {
				next = make(map[string]any)
				current[part] = next
			}
			current = next
		}
	}
}

func constructRumPayloadFromOTLP(attr pcommon.Map) map[string]any {
	rumPayload := make(map[string]any)
	attr.Range(func(k string, v pcommon.Value) bool {
		if rumAttributeName, exists := OTLPToRUMAttributeMap[k]; exists {
			buildRumPayload(rumAttributeName, v, rumPayload)
		}
		return true
	})
	return rumPayload
}
