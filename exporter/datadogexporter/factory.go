// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/metricsclient"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/trace/agent"
	"github.com/DataDog/datadog-agent/pkg/trace/telemetry"
	"github.com/DataDog/datadog-agent/pkg/trace/timing"
	"github.com/DataDog/datadog-agent/pkg/trace/writer"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

var logsAgentExporterFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.datadogexporter.UseLogsAgentExporter",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, datadogexporter uses the Datadog agent logs pipeline for exporting logs."),
	featuregate.WithRegisterFromVersion("v0.100.0"),
)

var metricExportNativeClientFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.datadogexporter.metricexportnativeclient",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("When enabled, metric export in datadogexporter uses native Datadog client APIs instead of Zorkian APIs."),
)

// noAPMStatsFeatureGate causes the trace consumer to skip APM stats computation.
var noAPMStatsFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.datadogexporter.DisableAPMStats",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("Datadog Exporter will not compute APM Stats"),
)

// isMetricExportV2Enabled returns true if metric export in datadogexporter uses native Datadog client APIs, false if it uses Zorkian APIs
func isMetricExportV2Enabled() bool {
	return metricExportNativeClientFeatureGate.IsEnabled()
}

func isLogsAgentExporterEnabled() bool {
	return logsAgentExporterFeatureGate.IsEnabled()
}

// enableNativeMetricExport switches metric export to call native Datadog APIs instead of Zorkian APIs.
func enableNativeMetricExport() error {
	return featuregate.GlobalRegistry().Set(metricExportNativeClientFeatureGate.ID(), true)
}

// enableZorkianMetricExport switches metric export to call Zorkian APIs instead of native Datadog APIs.
func enableZorkianMetricExport() error {
	return featuregate.GlobalRegistry().Set(metricExportNativeClientFeatureGate.ID(), false)
}

const metadataReporterPeriod = 30 * time.Minute

func consumeResource(metadataReporter *inframetadata.Reporter, res pcommon.Resource, logger *zap.Logger) {
	if err := metadataReporter.ConsumeResource(res); err != nil {
		logger.Warn("failed to consume resource for host metadata", zap.Error(err), zap.Any("resource", res))
	}
}

type factory struct {
	onceMetadata sync.Once

	onceProvider   sync.Once
	sourceProvider source.Provider
	providerErr    error

	onceReporter     sync.Once
	onceStopReporter sync.Once
	reporter         *inframetadata.Reporter
	reporterErr      error

	onceAttributesTranslator sync.Once
	attributesTranslator     *attributes.Translator
	attributesErr            error

	registry *featuregate.Registry
}

func (f *factory) SourceProvider(set component.TelemetrySettings, configHostname string, timeout time.Duration) (source.Provider, error) {
	f.onceProvider.Do(func() {
		f.sourceProvider, f.providerErr = hostmetadata.GetSourceProvider(set, configHostname, timeout)
	})
	return f.sourceProvider, f.providerErr
}

func (f *factory) AttributesTranslator(set component.TelemetrySettings) (*attributes.Translator, error) {
	f.onceAttributesTranslator.Do(func() {
		f.attributesTranslator, f.attributesErr = attributes.NewTranslator(set)
	})
	return f.attributesTranslator, f.attributesErr
}

// Reporter builds and returns an *inframetadata.Reporter.
func (f *factory) Reporter(params exporter.Settings, pcfg hostmetadata.PusherConfig) (*inframetadata.Reporter, error) {
	f.onceReporter.Do(func() {
		pusher := hostmetadata.NewPusher(params, pcfg)
		f.reporter, f.reporterErr = inframetadata.NewReporter(params.Logger, pusher, metadataReporterPeriod)
		if f.reporterErr == nil {
			go func() {
				if err := f.reporter.Run(context.Background()); err != nil {
					params.Logger.Error("Host metadata reporter failed at runtime", zap.Error(err))
				}
			}()
		}
	})
	return f.reporter, f.reporterErr
}

// StopReporter stops the host metadata reporter.
func (f *factory) StopReporter() {
	// Use onceReporter or wait until it is done
	f.onceReporter.Do(func() {})
	// Stop the reporter
	f.onceStopReporter.Do(func() {
		if f.reporterErr == nil && f.reporter != nil {
			f.reporter.Stop()
		}
	})
}

func (f *factory) TraceAgent(ctx context.Context, wg *sync.WaitGroup, params exporter.Settings, cfg *Config, sourceProvider source.Provider, attrsTranslator *attributes.Translator) (*agent.Agent, error) {
	agnt, err := newTraceAgent(ctx, params, cfg, sourceProvider, metricsclient.InitializeMetricClient(params.MeterProvider, metricsclient.ExporterSourceTag), attrsTranslator)
	if err != nil {
		return nil, err
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		agnt.Run()
	}()
	return agnt, nil
}

func newFactoryWithRegistry(registry *featuregate.Registry) exporter.Factory {
	f := &factory{registry: registry}
	return exporter.NewFactory(
		metadata.Type,
		f.createDefaultConfig,
		exporter.WithMetrics(f.createMetricsExporter, metadata.MetricsStability),
		exporter.WithTraces(f.createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(f.createLogsExporter, metadata.LogsStability),
	)
}

// NewFactory creates a Datadog exporter factory
func NewFactory() exporter.Factory {
	return newFactoryWithRegistry(featuregate.GlobalRegistry())
}

func defaultClientConfig() confighttp.ClientConfig {
	// do not use NewDefaultClientConfig for backwards-compatibility
	return confighttp.ClientConfig{
		Timeout: 15 * time.Second,
	}
}

// createDefaultConfig creates the default exporter configuration
func (f *factory) createDefaultConfig() component.Config {
	return &Config{
		ClientConfig:  defaultClientConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),

		API: APIConfig{
			Site: "datadoghq.com",
		},

		Metrics: MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://api.datadoghq.com",
			},
			DeltaTTL: 3600,
			ExporterConfig: MetricsExporterConfig{
				ResourceAttributesAsTags:           false,
				InstrumentationScopeMetadataAsTags: false,
			},
			HistConfig: HistogramConfig{
				Mode:             "distributions",
				SendAggregations: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
				InitialCumulativeMonotonicMode: InitialValueModeAuto,
			},
			SummaryConfig: SummaryConfig{
				Mode: SummaryModeGauges,
			},
		},

		Traces: TracesConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			IgnoreResources: []string{},
		},

		Logs: LogsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://http-intake.logs.datadoghq.com",
			},
			UseCompression:   true,
			CompressionLevel: 6,
			BatchWait:        5,
		},

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceConfigOrSystem,
		},
	}
}

// checkAndCastConfig checks the configuration type and its warnings, and casts it to
// the Datadog Config struct.
func checkAndCastConfig(c component.Config, logger *zap.Logger) *Config {
	cfg, ok := c.(*Config)
	if !ok {
		panic("programming error: config structure is not of type *datadogexporter.Config")
	}
	cfg.logWarnings(logger)
	return cfg
}

func (f *factory) consumeStatsPayload(ctx context.Context, wg *sync.WaitGroup, statsIn <-chan []byte, statsWriter *writer.DatadogStatsWriter, tracerVersion string, agentVersion string, logger *zap.Logger) {
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-statsIn:
					sp := &pb.StatsPayload{}

					err := proto.Unmarshal(msg, sp)
					if err != nil {
						logger.Error("failed to unmarshal stats payload", zap.Error(err))
						continue
					}
					for _, csp := range sp.Stats {
						if csp.TracerVersion == "" {
							csp.TracerVersion = tracerVersion
						}
					}
					// The DD Connector doesn't set the agent version, so we'll set it here
					sp.AgentVersion = agentVersion
					statsWriter.Write(sp)
				}
			}
		}()
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func (f *factory) createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	c component.Config,
) (exporter.Metrics, error) {
	cfg := checkAndCastConfig(c, set.TelemetrySettings.Logger)
	hostProvider, err := f.SourceProvider(set.TelemetrySettings, cfg.Hostname, cfg.HostMetadata.sourceTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to build hostname provider: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	// cancel() runs on shutdown

	attrsTranslator, err := f.AttributesTranslator(set.TelemetrySettings)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to build attributes translator: %w", err)
	}

	var (
		pushMetricsFn consumer.ConsumeMetricsFunc
		wg            sync.WaitGroup // waits for consumeStatsPayload to exit
	)

	acfg, err := newTraceAgentConfig(ctx, set, cfg, hostProvider, attrsTranslator)
	if err != nil {
		cancel()
		return nil, err
	}
	metricsClient := metricsclient.InitializeMetricClient(set.MeterProvider, metricsclient.ExporterSourceTag)
	timingReporter := timing.New(metricsClient)
	statsWriter := writer.NewStatsWriter(acfg, telemetry.NewNoopCollector(), metricsClient, timingReporter)

	set.Logger.Debug("Starting Datadog Trace-Agent StatsWriter")
	go statsWriter.Run()

	statsIn := make(chan []byte, 1000)
	statsv := set.BuildInfo.Command + set.BuildInfo.Version
	f.consumeStatsPayload(ctx, &wg, statsIn, statsWriter, statsv, acfg.AgentVersion, set.Logger)
	pcfg := newMetadataConfigfromConfig(cfg)
	metadataReporter, err := f.Reporter(set, pcfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to build host metadata reporter: %w", err)
	}

	if cfg.OnlyMetadata {
		pushMetricsFn = func(_ context.Context, md pmetric.Metrics) error {
			// only sending metadata use only metrics
			f.onceMetadata.Do(func() {
				attrs := pcommon.NewMap()
				if md.ResourceMetrics().Len() > 0 {
					attrs = md.ResourceMetrics().At(0).Resource().Attributes()
				}
				go hostmetadata.RunPusher(ctx, set, pcfg, hostProvider, attrs, metadataReporter)
			})

			// Consume resources for host metadata
			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				res := md.ResourceMetrics().At(i).Resource()
				consumeResource(metadataReporter, res, set.Logger)
			}
			return nil
		}
	} else {
		exp, metricsErr := newMetricsExporter(ctx, set, cfg, acfg, &f.onceMetadata, attrsTranslator, hostProvider, metadataReporter, statsIn)
		if metricsErr != nil {
			cancel()  // first cancel context
			wg.Wait() // then wait for shutdown
			return nil, metricsErr
		}
		pushMetricsFn = exp.PushMetricsDataScrubbed
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		pushMetricsFn,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0 * time.Second}),
		// We use our own custom mechanism for retries, since we hit several endpoints.
		exporterhelper.WithRetry(configretry.BackOffConfig{Enabled: false}),
		// The metrics remapping code mutates data
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithShutdown(func(context.Context) error {
			cancel()  // first cancel context
			wg.Wait() // then wait for shutdown
			f.StopReporter()
			statsWriter.Stop()
			if statsIn != nil {
				close(statsIn)
			}
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	return resourcetotelemetry.WrapMetricsExporter(
		resourcetotelemetry.Settings{Enabled: cfg.Metrics.ExporterConfig.ResourceAttributesAsTags}, exporter), nil
}

// createTracesExporter creates a trace exporter based on this config.
func (f *factory) createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	c component.Config,
) (exporter.Traces, error) {
	cfg := checkAndCastConfig(c, set.TelemetrySettings.Logger)
	if noAPMStatsFeatureGate.IsEnabled() {
		set.Logger.Info(
			"Trace metrics are now disabled in the Datadog Exporter by default. To continue receiving Trace Metrics, configure the Datadog Connector or disable the feature gate.",
			zap.String("documentation", "https://docs.datadoghq.com/opentelemetry/guide/migration/"),
			zap.String("feature gate ID", noAPMStatsFeatureGate.ID()),
		)
	}

	var (
		pusher consumer.ConsumeTracesFunc
		stop   component.ShutdownFunc
		wg     sync.WaitGroup // waits for agent to exit
	)

	hostProvider, err := f.SourceProvider(set.TelemetrySettings, cfg.Hostname, cfg.HostMetadata.sourceTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to build hostname provider: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	// cancel() runs on shutdown

	attrsTranslator, err := f.AttributesTranslator(set.TelemetrySettings)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to build attributes translator: %w", err)
	}

	traceagent, err := f.TraceAgent(ctx, &wg, set, cfg, hostProvider, attrsTranslator)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start trace-agent: %w", err)
	}

	pcfg := newMetadataConfigfromConfig(cfg)
	metadataReporter, err := f.Reporter(set, pcfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to build host metadata reporter: %w", err)
	}

	if cfg.OnlyMetadata {
		// only host metadata needs to be sent, once.
		pusher = func(_ context.Context, td ptrace.Traces) error {
			f.onceMetadata.Do(func() {
				attrs := pcommon.NewMap()
				if td.ResourceSpans().Len() > 0 {
					attrs = td.ResourceSpans().At(0).Resource().Attributes()
				}
				go hostmetadata.RunPusher(ctx, set, pcfg, hostProvider, attrs, metadataReporter)
			})
			// Consume resources for host metadata
			for i := 0; i < td.ResourceSpans().Len(); i++ {
				res := td.ResourceSpans().At(i).Resource()
				consumeResource(metadataReporter, res, set.Logger)
			}
			return nil
		}
		stop = func(context.Context) error {
			cancel()
			f.StopReporter()
			return nil
		}
	} else {
		tracex, err2 := newTracesExporter(ctx, set, cfg, &f.onceMetadata, hostProvider, traceagent, metadataReporter)
		if err2 != nil {
			cancel()
			wg.Wait() // then wait for shutdown
			return nil, err2
		}
		pusher = tracex.consumeTraces
		stop = func(context.Context) error {
			cancel() // first cancel context
			f.StopReporter()
			return nil
		}
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		pusher,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0 * time.Second}),
		// We don't do retries on traces because of deduping concerns on APM Events.
		exporterhelper.WithRetry(configretry.BackOffConfig{Enabled: false}),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithShutdown(stop),
	)
}

// createLogsExporter creates a logs exporter based on the config.
func (f *factory) createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	c component.Config,
) (exporter.Logs, error) {
	cfg := checkAndCastConfig(c, set.TelemetrySettings.Logger)

	var pusher consumer.ConsumeLogsFunc
	var logsAgent logsagentpipeline.LogsAgent
	hostProvider, err := f.SourceProvider(set.TelemetrySettings, cfg.Hostname, cfg.HostMetadata.sourceTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to build hostname provider: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	// cancel() runs on shutdown

	pcfg := newMetadataConfigfromConfig(cfg)
	metadataReporter, err := f.Reporter(set, pcfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to build host metadata reporter: %w", err)
	}

	attributesTranslator, err := f.AttributesTranslator(set.TelemetrySettings)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to build attributes translator: %w", err)
	}

	switch {
	case cfg.OnlyMetadata:
		// only host metadata needs to be sent, once.
		pusher = func(_ context.Context, td plog.Logs) error {
			f.onceMetadata.Do(func() {
				attrs := pcommon.NewMap()
				go hostmetadata.RunPusher(ctx, set, pcfg, hostProvider, attrs, metadataReporter)
			})
			for i := 0; i < td.ResourceLogs().Len(); i++ {
				res := td.ResourceLogs().At(i).Resource()
				consumeResource(metadataReporter, res, set.Logger)
			}
			return nil
		}
	case isLogsAgentExporterEnabled():
		la, exp, err := newLogsAgentExporter(ctx, set, cfg, hostProvider)
		if err != nil {
			cancel()
			return nil, err
		}
		logsAgent = la
		pusher = exp.ConsumeLogs
	default:
		exp, err := newLogsExporter(ctx, set, cfg, &f.onceMetadata, attributesTranslator, hostProvider, metadataReporter)
		if err != nil {
			cancel()
			return nil, err
		}
		pusher = exp.consumeLogs
	}
	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		pusher,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0 * time.Second}),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithShutdown(func(context.Context) error {
			cancel()
			f.StopReporter()
			if logsAgent != nil {
				return logsAgent.Stop(ctx)
			}
			return nil
		}),
	)
}
