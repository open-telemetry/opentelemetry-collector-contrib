// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	otlpmetrics "github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	zorkian "gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics/sketches"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
)

type metricsExporter struct {
	params           exporter.Settings
	cfg              *Config
	agntConfig       *config.AgentConfig
	ctx              context.Context
	client           *zorkian.Client
	metricsAPI       *datadogV2.MetricsApi
	tr               *otlpmetrics.Translator
	scrubber         scrub.Scrubber
	retrier          *clientutil.Retrier
	onceMetadata     *sync.Once
	sourceProvider   source.Provider
	metadataReporter *inframetadata.Reporter
	// getPushTime returns a Unix time in nanoseconds, representing the time pushing metrics.
	// It will be overwritten in tests.
	getPushTime func() uint64
}

// translatorFromConfig creates a new metrics translator from the exporter
func translatorFromConfig(set component.TelemetrySettings, cfg *Config, attrsTranslator *attributes.Translator, sourceProvider source.Provider, statsOut chan []byte) (*otlpmetrics.Translator, error) {
	options := []otlpmetrics.TranslatorOption{
		otlpmetrics.WithDeltaTTL(cfg.Metrics.DeltaTTL),
		otlpmetrics.WithFallbackSourceProvider(sourceProvider),
	}

	if isMetricRemappingDisabled() {
		set.Logger.Warn("Metric remapping is disabled in the Datadog exporter. OpenTelemetry metrics must be mapped to Datadog semantics before metrics are exported to Datadog (ex: via a processor).")
	} else {
		options = append(options, otlpmetrics.WithRemapping())
	}

	if cfg.Metrics.HistConfig.SendAggregations {
		options = append(options, otlpmetrics.WithHistogramAggregations())
	}

	if cfg.Metrics.SummaryConfig.Mode == SummaryModeGauges {
		options = append(options, otlpmetrics.WithQuantiles())
	}

	if cfg.Metrics.ExporterConfig.InstrumentationScopeMetadataAsTags {
		options = append(options, otlpmetrics.WithInstrumentationScopeMetadataAsTags())
	}

	options = append(options, otlpmetrics.WithHistogramMode(otlpmetrics.HistogramMode(cfg.Metrics.HistConfig.Mode)))

	var numberMode otlpmetrics.NumberMode
	switch cfg.Metrics.SumConfig.CumulativeMonotonicMode {
	case CumulativeMonotonicSumModeRawValue:
		numberMode = otlpmetrics.NumberModeRawValue
	case CumulativeMonotonicSumModeToDelta:
		numberMode = otlpmetrics.NumberModeCumulativeToDelta
	}
	options = append(options, otlpmetrics.WithNumberMode(numberMode))
	options = append(options, otlpmetrics.WithInitialCumulMonoValueMode(
		otlpmetrics.InitialCumulMonoValueMode(cfg.Metrics.SumConfig.InitialCumulativeMonotonicMode)))

	options = append(options, otlpmetrics.WithStatsOut(statsOut))
	return otlpmetrics.NewTranslator(set, attrsTranslator, options...)
}

func newMetricsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg *Config,
	agntConfig *config.AgentConfig,
	onceMetadata *sync.Once,
	attrsTranslator *attributes.Translator,
	sourceProvider source.Provider,
	metadataReporter *inframetadata.Reporter,
	statsOut chan []byte,
) (*metricsExporter, error) {
	tr, err := translatorFromConfig(params.TelemetrySettings, cfg, attrsTranslator, sourceProvider, statsOut)
	if err != nil {
		return nil, err
	}

	scrubber := scrub.NewScrubber()
	exporter := &metricsExporter{
		params:           params,
		cfg:              cfg,
		ctx:              ctx,
		agntConfig:       agntConfig,
		tr:               tr,
		scrubber:         scrubber,
		retrier:          clientutil.NewRetrier(params.Logger, cfg.BackOffConfig, scrubber),
		onceMetadata:     onceMetadata,
		sourceProvider:   sourceProvider,
		getPushTime:      func() uint64 { return uint64(time.Now().UTC().UnixNano()) },
		metadataReporter: metadataReporter,
	}
	errchan := make(chan error)
	if isMetricExportV2Enabled() {
		apiClient := clientutil.CreateAPIClient(
			params.BuildInfo,
			cfg.Metrics.TCPAddrConfig.Endpoint,
			cfg.ClientConfig)
		go func() { errchan <- clientutil.ValidateAPIKey(ctx, string(cfg.API.Key), params.Logger, apiClient) }()
		exporter.metricsAPI = datadogV2.NewMetricsApi(apiClient)
	} else {
		client := clientutil.CreateZorkianClient(string(cfg.API.Key), cfg.Metrics.TCPAddrConfig.Endpoint)
		client.ExtraHeader["User-Agent"] = clientutil.UserAgent(params.BuildInfo)
		client.HttpClient = clientutil.NewHTTPClient(cfg.ClientConfig)
		go func() { errchan <- clientutil.ValidateAPIKeyZorkian(params.Logger, client) }()
		exporter.client = client
	}
	if cfg.API.FailOnInvalidKey {
		err = <-errchan
		if err != nil {
			return nil, err
		}
	}
	return exporter, nil
}

func (exp *metricsExporter) pushSketches(ctx context.Context, sl sketches.SketchSeriesList) error {
	payload, err := sl.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal sketches: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		exp.cfg.Metrics.TCPAddrConfig.Endpoint+sketches.SketchSeriesEndpoint,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return fmt.Errorf("failed to build sketches HTTP request: %w", err)
	}

	clientutil.SetDDHeaders(req.Header, exp.params.BuildInfo, string(exp.cfg.API.Key))
	clientutil.SetExtraHeaders(req.Header, clientutil.ProtobufHeaders)
	var resp *http.Response
	if isMetricExportV2Enabled() {
		resp, err = exp.metricsAPI.Client.Cfg.HTTPClient.Do(req)
	} else {
		resp, err = exp.client.HttpClient.Do(req)
	}

	if err != nil {
		return clientutil.WrapError(fmt.Errorf("failed to do sketches HTTP request: %w", err), resp)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return clientutil.WrapError(fmt.Errorf("error when sending payload to %s: %s", sketches.SketchSeriesEndpoint, resp.Status), resp)
	}
	return nil
}

func (exp *metricsExporter) PushMetricsDataScrubbed(ctx context.Context, md pmetric.Metrics) error {
	return exp.scrubber.Scrub(exp.PushMetricsData(ctx, md))
}

func (exp *metricsExporter) PushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	if exp.cfg.HostMetadata.Enabled {
		// Start host metadata with resource attributes from
		// the first payload.
		exp.onceMetadata.Do(func() {
			attrs := pcommon.NewMap()
			if md.ResourceMetrics().Len() > 0 {
				attrs = md.ResourceMetrics().At(0).Resource().Attributes()
			}
			go hostmetadata.RunPusher(exp.ctx, exp.params, newMetadataConfigfromConfig(exp.cfg), exp.sourceProvider, attrs, exp.metadataReporter)
		})

		// Consume resources for host metadata
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			res := md.ResourceMetrics().At(i).Resource()
			consumeResource(exp.metadataReporter, res, exp.params.Logger)
		}
	}
	var consumer otlpmetrics.Consumer
	if isMetricExportV2Enabled() {
		consumer = metrics.NewConsumer()
	} else {
		consumer = metrics.NewZorkianConsumer()
	}
	metadata, err := exp.tr.MapMetrics(ctx, md, consumer)
	if err != nil {
		return fmt.Errorf("failed to map metrics: %w", err)
	}
	src, err := exp.sourceProvider.Source(ctx)
	if err != nil {
		return err
	}
	var tags []string
	if src.Kind == source.AWSECSFargateKind {
		tags = append(tags, exp.cfg.HostMetadata.Tags...)
	}

	var sl sketches.SketchSeriesList
	var errs []error
	if isMetricExportV2Enabled() {
		var ms []datadogV2.MetricSeries
		ms, sl = consumer.(*metrics.Consumer).All(exp.getPushTime(), exp.params.BuildInfo, tags, metadata)
		if len(ms) > 0 {
			exp.params.Logger.Debug("exporting native Datadog payload", zap.Any("metric", ms))
			_, experr := exp.retrier.DoWithRetries(ctx, func(context.Context) error {
				ctx = clientutil.GetRequestContext(ctx, string(exp.cfg.API.Key))
				_, httpresp, merr := exp.metricsAPI.SubmitMetrics(ctx, datadogV2.MetricPayload{Series: ms}, *clientutil.GZipSubmitMetricsOptionalParameters)
				return clientutil.WrapError(merr, httpresp)
			})
			errs = append(errs, experr)
		}
	} else {
		var ms []zorkian.Metric
		ms, sl = consumer.(*metrics.ZorkianConsumer).All(exp.getPushTime(), exp.params.BuildInfo, tags)
		if len(ms) > 0 {
			exp.params.Logger.Debug("exporting Zorkian Datadog payload", zap.Any("metric", ms))
			_, experr := exp.retrier.DoWithRetries(ctx, func(context.Context) error {
				return exp.client.PostMetrics(ms)
			})
			errs = append(errs, experr)
		}
	}

	if len(sl) > 0 {
		exp.params.Logger.Debug("exporting sketches payload", zap.Any("sketches", sl))
		_, experr := exp.retrier.DoWithRetries(ctx, func(ctx context.Context) error {
			return exp.pushSketches(ctx, sl)
		})
		errs = append(errs, experr)
	}

	return errors.Join(errs...)
}
