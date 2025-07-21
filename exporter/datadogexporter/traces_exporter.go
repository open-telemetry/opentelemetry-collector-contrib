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
					fmt.Println("&&&&&&&&&& CONVERTING TO RUM: ")

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

					exp.params.Logger.Info("RUM SPAN ATTRIBUTES%%%%%%%%%%%%%%", zap.Any("sattr", sattr.AsRaw()))
					// construct request body according to RUM spec
					rumPayload := constructRumPayloadFromOTLP(sattr)
					exp.params.Logger.Info("RUM Payload%%%%%%%%%%%%%%", zap.Any("rumPayload", rumPayload))

					byts, err := json.Marshal(rumPayload)
					prettybyts, _ := json.MarshalIndent(rumPayload, "", "  ")
					exp.params.Logger.Info("PRETTY RUM Payload%%%%%%%%%%%%%%", zap.Any("prettybyts", string(prettybyts)))
					if err != nil {
						return fmt.Errorf("failed to marshal RUM payload: %v", err)
					}
					req.Body = io.NopCloser(bytes.NewBuffer(byts))
					exp.params.Logger.Info("RUM PAYLOAD BODY%%%%%%%%%%%%%%", zap.Any("byts", string(byts)))
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
			if v.Type() == pcommon.ValueTypeSlice || v.Type() == pcommon.ValueTypeMap {
				current[part] = v.AsString()
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

var OTLPToRUMAttributeMap = map[string]string{
	// _common-schema.json
	"datadog.date":                                         "date",
	"datadog.application.id":                               "application.id",
	"datadog.application.current_locale":                   "application.current_locale",
	"service.name":                                         "service",
	"service.version":                                      "version",
	"datadog.build_version":                                "build_version",
	"datadog.build_id":                                     "build_id",
	"session.id":                                           "session.id",
	"datadog.session.type":                                 "session.type",
	"datadog.session.has_replay":                           "session.has_replay",
	"datadog.source":                                       "source",
	"datadog.view.id":                                      "view.id",
	"datadog.view.referrer":                                "view.referrer",
	"datadog.view.url":                                     "view.url",
	"datadog.view.name":                                    "view.name",
	"user.id":                                              "usr.id",
	"user.full_name":                                       "usr.name",
	"user.email":                                           "usr.email",
	"user.hash":                                            "usr.anonymoud_id",
	"datadog.account.id":                                   "account.id",
	"user.name":                                            "account.name",
	"datadog.connectivity.status":                          "connectivity.status",
	"datadog.connectivity.interfaces":                      "connectivity.interfaces",
	"datadog.connectivity.effective_type":                  "connectivity.effective_type",
	"datadog.connectivity.cellular.technology":             "connectivity.cellular.technology",
	"datadog.connectivity.cellular.carrier_name":           "connectivity.cellular.carrier_name",
	"datadog.display.viewport.width":                       "display.viewport.width",
	"datadog.display.viewport.height":                      "display.viewport.height",
	"datadog.synthetics.test_id":                           "synthetics.test_id",
	"datadog.synthetics.result_id":                         "synthetics.result_id",
	"datadog.synthetics.injected":                          "synthetics.injected",
	"datadog.ci_test.test_execution_id":                    "ci_test.test_execution_id",
	"os.name":                                              "os.name",
	"os.version":                                           "os.version",
	"os.build_id":                                          "os.build",
	"datadog.os.version_major":                             "os.version_major",
	"datadog.device.type":                                  "device.type",
	"device.model.name":                                    "device.name",
	"device.model.identifier":                              "device.model",
	"device.manufacturer":                                  "device.brand",
	"datadog.device.architecture":                          "device.architecture",
	"datadog.device.locale":                                "device.locale",
	"datadog.device.locales":                               "device.locales",
	"datadog.device.time_zone":                             "device.time_zone",
	"datadog.device.battery_level":                         "device.battery_level",
	"datadog.device.power_saving_mode":                     "device.power_saving_mode",
	"datadog.device.brightness_level":                      "device.brightness_level",
	"datadog._dd.format_version":                           "_dd.format_version",
	"datadog._dd.session.plan":                             "_dd.session.plan",
	"datadog._dd.session.session_precondition":             "_dd.session.session_precondition",
	"datadog._dd.configuration.session_sample_rate":        "_dd.configuration.session_sample_rate",
	"datadog._dd.configuration.session_replay_sample_rate": "_dd.configuration.session_replay_sample_rate",
	"datadog._dd.configuration.profiling_sample_rate":      "_dd.configuration.profiling_sample_rate",
	"datadog._dd.browser_sdk_version":                      "_dd.browser_sdk_version",
	"datadog._dd.sdk_name":                                 "_dd.sdk_name",
	"datadog.context":                                      "context",

	// _view-container-schema.json
	"datadog.container.view.id": "container.view.id",
	"datadog.container.source":  "container.source",

	// _action-child-schema.json
	"datadog.action.id": "action.id",

	// action-schema.json
	"datadog.action.type":                "action.type",
	"datadog.action.loading_time":        "action.loading_time",
	"datadog.action.target.name":         "action.target.name",
	"datadog.action.frustration.type":    "action.frustration.type",
	"datadog.action.error.count":         "action.error.count",
	"datadog.action.crash.count":         "action.crash.count",
	"datadog.action.long_task.count":     "action.long_task.count",
	"datadog.action.resource.count":      "action.resource.count",
	"datadog.view.in_foreground":         "view.in_foreground",
	"datadog._dd.action.position.x":      "_dd.action.position.x",
	"datadog._dd.action.position.y":      "_dd.action.position.y",
	"datadog._dd.action.target.selector": "_dd.action.target.selector",
	"datadog._dd.action.target.width":    "_dd.action.target.width",
	"datadog._dd.action.target.height":   "_dd.action.target.height",
	"datadog._dd.action.name_source":     "_dd.action.name_source",

	// error-schema.json
	"datadog.error.id":                         "error.id",
	"error.message":                            "error.message",
	"datadog.error.source":                     "error.source",
	"datadog.error.stack":                      "error.stack",
	"datadog.error.causes":                     "error.causes",
	"datadog.error.causes.message":             "error.causes.message",
	"datadog.error.causes.type":                "error.causes.type",
	"datadog.error.causes.stack":               "error.causes.stack",
	"datadog.error.causes.source":              "error.causes.source",
	"datadog.error.is_crash":                   "error.is_crash",
	"datadog.error.fingerprint":                "error.fingerprint",
	"error.type":                               "error.type",
	"datadog.error.category":                   "error.category",
	"datadog.error.handling":                   "error.handling",
	"datadog.error.handling_stack":             "error.handling_stack",
	"datadog.error.source_type":                "error.source_type",
	"datadog.error.resource.method":            "error.resource.method",
	"datadog.error.resource.status_code":       "error.resource.status_code",
	"datadog.error.resource.url":               "error.resource.url",
	"datadog.error.resource.provider.domain":   "error.resource.provider.domain",
	"datadog.error.resource.provider.name":     "error.resource.provider.name",
	"datadog.error.resource.provider.type":     "error.resource.provider.type",
	"datadog.error.threads":                    "error.threads",
	"datadog.error.threads.name":               "error.threads.name",
	"datadog.error.threads.crashed":            "error.threads.crashed",
	"datadog.error.threads.stack":              "error.threads.stack",
	"datadog.error.threads.state":              "error.threads.state",
	"datadog.error.binary_images":              "error.binary_images",
	"datadog.error.binary_images.uuid":         "error.binary_images.uuid",
	"datadog.error.binary_images.name":         "error.binary_images.name",
	"datadog.error.binary_images.is_system":    "error.binary_images.is_system",
	"datadog.error.binary_images.load_address": "error.binary_images.load_address",
	"datadog.error.binary_images.max_address":  "error.binary_images.max_address",
	"datadog.error.binary_images.arch":         "error.binary_images.arch",
	"datadog.error.was_truncated":              "error.was_truncated",
	"datadog.error.meta.code_type":             "error.meta.code_type",
	"datadog.error.meta.parent_process":        "error.meta.parent_process",
	"datadog.error.meta.incident_identifier":   "error.meta.incident_identifier",
	"datadog.error.meta.process":               "error.meta.process",
	"datadog.error.meta.exception_type":        "error.meta.exception_type",
	"datadog.error.meta.exception_codes":       "error.meta.exception_codes",
	"datadog.error.meta.path":                  "error.meta.path",
	"datadog.error.csp.disposition":            "error.csp.disposition",
	"datadog.error.time_since_app_start":       "error.time_since_app_start",
	"datadog.freeze.duration":                  "freeze.duration",
	"datadog.feature_flags":                    "feature_flags",

	// long_task-schema.json
	"datadog.long_task.id":                                       "long_task.id",
	"datadog.long_task.start_time":                               "long_task.start_time",
	"datadog.long_task.entry_type":                               "long_task.entry_type",
	"datadog.long_task.duration":                                 "long_task.duration",
	"datadog.long_task.blocking_duration":                        "long_task.blocking_duration",
	"datadog.long_task.render_start":                             "long_task.render_start",
	"datadog.long_task.style_and_layout_start":                   "long_task.style_and_layout_start",
	"datadog.long_task.first_ui_event_timestamp":                 "long_task.first_ui_event_timestamp",
	"datadog.long_task.is_frozen_frame":                          "long_task.is_frozen_frame",
	"datadog.long_task.scripts":                                  "long_task.scripts",
	"datadog.long_task.scripts.duration":                         "long_task.scripts.duration",
	"datadog.long_task.scripts.pause_duration":                   "long_task.scripts.pause_duration",
	"datadog.long_task.scripts.forced_style_and_layout_duration": "long_task.scripts.forced_style_and_layout_duration",
	"datadog.long_task.scripts.start_time":                       "long_task.scripts.start_time",
	"datadog.long_task.scripts.execution_start":                  "long_task.scripts.execution_start",
	"datadog.long_task.scripts.souce_url":                        "long_task.scripts.souce_url",
	"datadog.long_task.scripts.source_function_name":             "long_task.scripts.source_function_name",
	"datadog.long_task.scripts.source_char_position":             "long_task.scripts.source_char_position",
	"datadog.long_task.scripts.invoker":                          "long_task.scripts.invoker",
	"datadog.long_task.scripts.invoker_type":                     "long_task.scripts.invoker_type",
	"datadog.long_task.scripts.window_attribution":               "long_task.scripts.window_attribution",
	"datadog._dd.discarded":                                      "_dd.discarded",
	"datadog._dd.profiling":                                      "_dd.profiling",

	// resource-schema.json
	"datadog.resource.id":                     "resource.id",
	"datadog.resource.type":                   "resource.type",
	"datadog.resource.method":                 "resource.method",
	"datadog.resource.url":                    "resource.url",
	"datadog.resource.status_code":            "resource.status_code",
	"datadog.resource.duration":               "resource.duration",
	"datadog.resource.size":                   "resource.size",
	"datadog.resource.encoded_body_size":      "resource.encoded_body_size",
	"datadog.resource.decoded_body_size":      "resource.decoded_body_size",
	"datadog.resource.transfer_size":          "resource.transfer_size",
	"datadog.resource.render_blocking_status": "resource.render_blocking_status",
	"datadog.resource.worker.duration":        "resource.worker.duration",
	"datadog.resource.worker.start":           "resource.worker.start",
	"datadog.resource.redirect.duration":      "resource.redirect.duration",
	"datadog.resource.redirect.start":         "resource.redirect.start",
	"datadog.resource.dns.duration":           "resource.dns.duration",
	"datadog.resource.dns.start":              "resource.dns.start",
	"datadog.resource.connect.duration":       "resource.connect.duration",
	"datadog.resource.connect.start":          "resource.connect.start",
	"datadog.resource.ssl.duration":           "resource.ssl.duration",
	"datadog.resource.ssl.start":              "resource.ssl.start",
	"datadog.resource.first_byte.duration":    "resource.first_byte.duration",
	"datadog.resource.first_byte.start":       "resource.first_byte.start",
	"datadog.resource.download.duration":      "resource.download.duration",
	"datadog.resource.download.start":         "resource.download.start",
	"datadog.resource.protocol":               "resource.protocol",
	"datadog.resource.delivery_type":          "resource.delivery_type",
	"datadog.resource.provider.domain":        "resource.provider.domain",
	"datadog.resource.provider.name":          "resource.provider.name",
	"datadog.resource.provider.type":          "resource.provider.type",
	"datadog.resource.graphql.operationType":  "resource.graphql.operationType",
	"datadog.resource.graphql.operationName":  "resource.graphql.operationName",
	"datadog.resource.graphql.payload":        "resource.graphql.payload",
	"datadog.resource.graphql.variables":      "resource.graphql.variables",
	"datadog._dd.span_id":                     "_dd.span_id",
	"datadog._dd.parent_span_id":              "_dd.parent_span_id",
	"datadog._dd.trace_id":                    "_dd.trace_id",
	"datadog._dd.rule_psr":                    "_dd.rule_psr",
	"datadog._dd.profiling.status":            "_dd.profiling.status",
	"datadog._dd.profiling.error_reason":      "_dd.profiling.error_reason",

	// _view-schema.json
	"datadog.view.loading_time":                                         "view.loading_time",
	"datadog.view.network_settled_time":                                 "view.network_settled_time",
	"datadog.view.interaction_to_next_view_time":                        "view.interaction_to_next_view_time",
	"datadog.view.loading_type":                                         "view.loading_type",
	"datadog.view.time_spent":                                           "view.time_spent",
	"datadog.view.first_contentful_paint":                               "view.first_contentful_paint",
	"datadog.view.largest_contentful_paint":                             "view.largest_contentful_paint",
	"datadog.view.largest_contentful_paint_target_selector":             "view.largest_contentful_paint_target_selector",
	"datadog.view.first_input_delay":                                    "view.first_input_delay",
	"datadog.view.first_input_time":                                     "view.first_input_time",
	"datadog.view.first_input_target_selector":                          "view.first_input_target_selector",
	"datadog.view.interaction_to_next_paint":                            "view.interaction_to_next_paint",
	"datadog.view.interaction_to_next_paint_time":                       "view.interaction_to_next_paint_time",
	"datadog.view.interaction_to_next_paint_target_selector":            "view.interaction_to_next_paint_target_selector",
	"datadog.view.cumulative_layout_shift":                              "view.cumulative_layout_shift",
	"datadog.view.cumulative_layout_shift_time":                         "view.cumulative_layout_shift_time",
	"datadog.view.cumulative_layout_shift_target_selector":              "view.cumulative_layout_shift_target_selector",
	"datadog.view.dom_complete":                                         "view.dom_complete",
	"datadog.view.dom_content_loaded":                                   "view.dom_content_loaded",
	"datadog.view.dom_interactive":                                      "view.dom_interactive",
	"datadog.view.load_event":                                           "view.load_event",
	"datadog.view.first_byte":                                           "view.first_byte",
	"datadog.view.custom_timings":                                       "view.custom_timings",
	"datadog.view.is_active":                                            "view.is_active",
	"datadog.view.is_slow_rendered":                                     "view.is_slow_rendered",
	"datadog.view.action.count":                                         "view.action.count",
	"datadog.view.error.count":                                          "view.error.count",
	"datadog.view.crash.count":                                          "view.crash.count",
	"datadog.view.long_task.count":                                      "view.long_task.count",
	"datadog.view.frozen_frame.count":                                   "view.frozen_frame.count",
	"datadog.view.slow_frames":                                          "view.slow_frames",
	"datadog.view.slow_frames.start":                                    "view.slow_frames.start",
	"datadog.view.slow_frames.duration":                                 "view.slow_frames.duration",
	"datadog.view.resource.count":                                       "view.resource.count",
	"datadog.view.frustration.count":                                    "view.frustration.count",
	"datadog.view.in_foreground_periods":                                "view.in_foreground_periods",
	"datadog.view.in_foreground_periods.start":                          "view.in_foreground_periods.start",
	"datadog.view.in_foreground_periods.duration":                       "view.in_foreground_periods.duration",
	"datadog.view.memory_average":                                       "view.memory_average",
	"datadog.view.memory_max":                                           "view.memory_max",
	"datadog.view.cpu_ticks_count":                                      "view.cpu_ticks_count",
	"datadog.view.cpu_ticks_per_second":                                 "view.cpu_ticks_per_second",
	"datadog.view.refresh_rate_average":                                 "view.refresh_rate_average",
	"datadog.view.refresh_rate_min":                                     "view.refresh_rate_min",
	"datadog.view.slow_frames_rate":                                     "view.slow_frames_rate",
	"datadog.view.freeze_rate":                                          "view.freeze_rate",
	"datadog.view.flutter_build_time.min":                               "view.flutter_build_time.min",
	"datadog.view.flutter_build_time.max":                               "view.flutter_build_time.max",
	"datadog.view.flutter_build_time.average":                           "view.flutter_build_time.average",
	"datadog.view.flutter_build_time.metric_max":                        "view.flutter_build_time.metric_max",
	"datadog.view.flutter_raster_time.min":                              "view.flutter_raster_time.min",
	"datadog.view.flutter_raster_time.max":                              "view.flutter_raster_time.max",
	"datadog.view.flutter_raster_time.average":                          "view.flutter_raster_time.average",
	"datadog.view.flutter_raster_time.metric_max":                       "view.flutter_raster_time.metric_max",
	"datadog.view.js_refresh_rate.min":                                  "view.js_refresh_rate.min",
	"datadog.view.js_refresh_rate.max":                                  "view.js_refresh_rate.max",
	"datadog.view.js_refresh_rate.average":                              "view.js_refresh_rate.average",
	"datadog.view.js_refresh_rate.metric_max":                           "view.js_refresh_rate.metric_max",
	"datadog.view.performance.cls.score":                                "view.performance.cls.score",
	"datadog.view.performance.cls.timestamp":                            "view.performance.cls.timestamp",
	"datadog.view.performance.cls.target_selector":                      "view.performance.cls.target_selector",
	"datadog.view.performance.cls.previous_rect.x":                      "view.performance.cls.previous_rect.x",
	"datadog.view.performance.cls.previous_rect.y":                      "view.performance.cls.previous_rect.y",
	"datadog.view.performance.cls.previous_rect.width":                  "view.performance.cls.previous_rect.width",
	"datadog.view.performance.cls.previous_rect.height":                 "view.performance.cls.previous_rect.height",
	"datadog.view.performance.cls.current_rect.x":                       "view.performance.cls.current_rect.x",
	"datadog.view.performance.cls.current_rect.y":                       "view.performance.cls.current_rect.y",
	"datadog.view.performance.cls.current_rect.width":                   "view.performance.cls.current_rect.width",
	"datadog.view.performance.cls.current_rect.height":                  "view.performance.cls.current_rect.height",
	"datadog.view.performance.fcp.timestamp":                            "view.performance.fcp.timestamp",
	"datadog.view.performance.fid.duration":                             "view.performance.fid.duration",
	"datadog.view.performance.fid.timestamp":                            "view.performance.fid.timestamp",
	"datadog.view.performance.fid.target_selector":                      "view.performance.fid.target_selector",
	"datadog.view.performance.inp.duration":                             "view.performance.inp.duration",
	"datadog.view.performance.inp.timestamp":                            "view.performance.inp.timestamp",
	"datadog.view.performance.inp.target_selector":                      "view.performance.inp.target_selector",
	"datadog.view.performance.lcp.timestamp":                            "view.performance.lcp.timestamp",
	"datadog.view.performance.lcp.target_selector":                      "view.performance.lcp.target_selector",
	"datadog.view.performance.lcp.resource_url":                         "view.performance.lcp.resource_url",
	"datadog.view.performance.fbc.timestamp":                            "view.performance.fbc.timestamp",
	"datadog.session.is_active":                                         "session.is_active",
	"datadog.session.sampled_for_replay":                                "session.sampled_for_replay",
	"datadog.privacy.replay_level":                                      "privacy.replay_level",
	"datadog._dd.document_version":                                      "_dd.document_version",
	"datadog._dd.page_states":                                           "_dd.page_states",
	"datadog._dd.page_states.state":                                     "_dd.page_states.state",
	"datadog._dd.page_states.start":                                     "_dd.page_states.start",
	"datadog._dd.replay_stats.records_count":                            "_dd.replay_stats.records_count",
	"datadog._dd.replay_stats.segments_count":                           "_dd.replay_stats.segments_count",
	"datadog._dd.replay_stats.segments_total_raw_size":                  "_dd.replay_stats.segments_total_raw_size",
	"datadog._dd.cls.device_pixel_ratio":                                "_dd.cls.device_pixel_ratio",
	"datadog._dd.configuration.start_session_replay_recording_manually": "_dd.configuration.start_session_replay_recording_manually",
	"datadog.display.scroll.max_depth":                                  "display.scroll.max_depth",
	"datadog.display.scroll.max_depth_scroll_top":                       "display.scroll.max_depth_scroll_top",
	"datadog.display.scroll.max_scroll_height":                          "display.scroll.max_scroll_height",
	"datadog.display.scroll.max_scroll_height_time":                     "display.scroll.max_scroll_height_time",

	// vitals-schema.json
	"datadog.vital.type":               "vital.type",
	"datadog.vital.id":                 "vital.id",
	"datadog.vital.name":               "vital.name",
	"datadog.vital.description":        "vital.description",
	"datadog.vital.duration":           "vital.duration",
	"datadog.vital.custom":             "vital.custom",
	"datadog._dd.vital.computed_value": "_dd.vital.computed_value",
}
