// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/statsprocessor"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	traceconfig "github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/DataDog/datadog-agent/pkg/trace/timing"
	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"github.com/patrickmn/go-cache"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// traceToMetricConnector is the schema for connector
type traceToMetricConnector struct {
	metricsConsumer consumer.Metrics // the next component in the pipeline to ingest metrics after connector
	logger          *zap.Logger

	// agent specifies the agent used to ingest traces and output APM Stats.
	// It is implemented by the traceagent structure; replaced in tests.
	agent statsprocessor.Ingester

	// translator specifies the translator used to transform APM Stats Payloads
	// from the agent to OTLP Metrics.
	translator *metrics.Translator

	enrichedTags      map[string]string
	containerTagCache *cache.Cache

	// in specifies the channel through which the agent will output Stats Payloads
	// resulting from ingested traces.
	in chan *pb.StatsPayload

	// exit specifies the exit channel, which will be closed upon shutdown.
	exit chan struct{}

	// isStarted tracks whether Start() has been called.
	isStarted bool
}

var _ component.Component = (*traceToMetricConnector)(nil) // testing that the connectorImp properly implements the type Component interface

// cacheExpiration is the time after which a container tag cache entry will expire
// and be removed from the cache.
var cacheExpiration = time.Minute * 5

// cacheCleanupInterval is the time after which the cache will be cleaned up.
var cacheCleanupInterval = time.Minute

// function to create a new connector
func newTraceToMetricConnector(set component.TelemetrySettings, cfg component.Config, metricsConsumer consumer.Metrics, metricsClient statsd.ClientInterface, timingReporter timing.Reporter) (*traceToMetricConnector, error) {
	set.Logger.Info("Building datadog connector for traces to metrics")
	in := make(chan *pb.StatsPayload, 100)
	set.MeterProvider = noop.NewMeterProvider() // disable metrics for the connector
	attributesTranslator, err := attributes.NewTranslator(set)
	if err != nil {
		return nil, fmt.Errorf("failed to create attributes translator: %w", err)
	}
	trans, err := metrics.NewTranslator(set, attributesTranslator)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics translator: %w", err)
	}

	ctags := make(map[string]string, len(cfg.(*Config).Traces.ResourceAttributesAsContainerTags))
	for _, val := range cfg.(*Config).Traces.ResourceAttributesAsContainerTags {
		if v, ok := attributes.ContainerMappings[val]; ok {
			ctags[v] = ""
		} else {
			ctags[val] = ""
		}
	}
	ctx := context.Background()
	return &traceToMetricConnector{
		logger:            set.Logger,
		agent:             statsprocessor.NewAgentWithConfig(ctx, getTraceAgentCfg(set.Logger, cfg.(*Config).Traces, attributesTranslator), in, metricsClient, timingReporter),
		translator:        trans,
		in:                in,
		metricsConsumer:   metricsConsumer,
		enrichedTags:      ctags,
		containerTagCache: cache.New(cacheExpiration, cacheCleanupInterval),
		exit:              make(chan struct{}),
	}, nil
}

func getTraceAgentCfg(logger *zap.Logger, cfg datadogconfig.TracesConnectorConfig, attributesTranslator *attributes.Translator) *traceconfig.AgentConfig {
	acfg := traceconfig.New()
	acfg.OTLPReceiver.AttributesTranslator = attributesTranslator
	acfg.OTLPReceiver.SpanNameRemappings = cfg.SpanNameRemappings
	acfg.OTLPReceiver.SpanNameAsResourceName = cfg.SpanNameAsResourceName
	acfg.Ignore["resource"] = cfg.IgnoreResources
	acfg.ComputeStatsBySpanKind = cfg.ComputeStatsBySpanKind
	acfg.PeerTagsAggregation = cfg.PeerTagsAggregation
	acfg.PeerTags = cfg.PeerTags
	if len(cfg.ResourceAttributesAsContainerTags) > 0 {
		acfg.Features["enable_cid_stats"] = struct{}{}
		delete(acfg.Features, "disable_cid_stats")
	}
	if v := cfg.TraceBuffer; v > 0 {
		acfg.TraceBuffer = v
	}
	if cfg.ComputeTopLevelBySpanKind {
		logger.Info("traces::compute_top_level_by_span_kind needs to be enabled in both the Datadog connector and Datadog exporter configs if both components are being used")
		acfg.Features["enable_otlp_compute_top_level_by_span_kind"] = struct{}{}
	}
	if !datadog.ReceiveResourceSpansV2FeatureGate.IsEnabled() {
		acfg.Features["disable_receive_resource_spans_v2"] = struct{}{}
	}
	if datadog.OperationAndResourceNameV2FeatureGate.IsEnabled() {
		acfg.Features["enable_operation_and_resource_name_logic_v2"] = struct{}{}
	} else {
		logger.Info("Please enable feature gate datadog.EnableOperationAndResourceNameV2 for improved operation and resource name logic. This flag will be made stable in a future release. If you have Datadog monitors or alerts set on operation/resource names, you may need to migrate them to the new convention. See the migration guide at https://docs.datadoghq.com/opentelemetry/guide/migrate/migrate_operation_names/")
	}
	if v := cfg.BucketInterval; v > 0 {
		acfg.BucketInterval = v
	}
	return acfg
}

// Start implements the component.Component interface.
func (c *traceToMetricConnector) Start(_ context.Context, _ component.Host) error {
	c.logger.Info("Starting datadogconnector")
	c.agent.Start()
	go c.run()
	c.isStarted = true
	return nil
}

// Shutdown implements the component.Component interface.
func (c *traceToMetricConnector) Shutdown(context.Context) error {
	if !c.isStarted {
		// Note: it is not necessary to manually close c.exit, c.in and c.agent.(*statsprocessor.TraceAgent).exit channels as these are unused.
		c.logger.Info("Requested shutdown, but not started, ignoring.")
		return nil
	}
	c.logger.Info("Shutting down datadog connector")
	c.logger.Info("Stopping datadog agent")
	// stop the agent and wait for the run loop to exit
	c.agent.Stop()
	c.exit <- struct{}{} // signal exit
	<-c.exit             // wait for close
	return nil
}

// Capabilities implements the consumer interface.
// tells use whether the component(connector) will mutate the data passed into it. if set to true the connector does modify the data
func (c *traceToMetricConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *traceToMetricConnector) addToCache(containerID string, key string) {
	if tags, ok := c.containerTagCache.Get(containerID); ok {
		tagList := tags.(*sync.Map)
		tagList.Store(key, struct{}{})
	} else {
		tagList := &sync.Map{}
		tagList.Store(key, struct{}{})
		c.containerTagCache.Set(containerID, tagList, cache.DefaultExpiration)
	}
}

func (c *traceToMetricConnector) populateContainerTagsCache(traces ptrace.Traces) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		attrs := rs.Resource().Attributes()

		containerID, ok := attrs.Get(string(semconv.ContainerIDKey))
		if !ok {
			continue
		}
		ddContainerTags := attributes.ContainerTagsFromResourceAttributes(attrs)
		for attr := range c.enrichedTags {
			if val, ok := ddContainerTags[attr]; ok {
				key := fmt.Sprintf("%s:%s", attr, val)
				c.addToCache(containerID.AsString(), key)
			} else if incomingVal, ok := attrs.Get(attr); ok {
				key := fmt.Sprintf("%s:%s", attr, incomingVal.Str())
				c.addToCache(containerID.AsString(), key)
			}
		}
	}
}

func (c *traceToMetricConnector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	c.populateContainerTagsCache(traces)
	c.agent.Ingest(ctx, traces)
	return nil
}

func (c *traceToMetricConnector) enrichStatsPayload(stats *pb.StatsPayload) {
	for _, stat := range stats.Stats {
		if stat.ContainerID != "" {
			if tags, ok := c.containerTagCache.Get(stat.ContainerID); ok {
				tagList := tags.(*sync.Map)
				for _, tag := range stat.Tags {
					tagList.Store(tag, struct{}{})
				}
				stat.Tags = make([]string, 0)
				tagList.Range(func(key, _ any) bool {
					stat.Tags = append(stat.Tags, key.(string))
					return true
				})
			}
		}
	}
}

// run awaits incoming stats resulting from the agent's ingestion, converts them
// to metrics and flushes them using the configured metrics exporter.
func (c *traceToMetricConnector) run() {
	defer close(c.exit)
	for {
		select {
		case stats := <-c.in:
			if len(stats.Stats) == 0 {
				continue
			}
			var mx pmetric.Metrics
			var err error
			// Enrich the stats with container tags
			if len(c.enrichedTags) > 0 {
				c.enrichStatsPayload(stats)
			}

			c.logger.Debug("Received stats payload", zap.Any("stats", stats))

			mx, err = c.translator.StatsToMetrics(stats)
			if err != nil {
				c.logger.Error("Failed to convert stats to metrics", zap.Error(err))
				continue
			}
			// APM stats as metrics
			ctx := context.TODO()

			// send metrics to the consumer or next component in pipeline
			if err := c.metricsConsumer.ConsumeMetrics(ctx, mx); err != nil {
				c.logger.Error("Failed ConsumeMetrics", zap.Error(err))
				return
			}
		case <-c.exit:
			return
		}
	}
}
