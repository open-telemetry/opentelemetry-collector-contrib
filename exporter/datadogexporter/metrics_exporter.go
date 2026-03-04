// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	otlpmetrics "github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics"
	"github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics/sketches"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/scrub"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/featuregates"
)

type metricsExporter struct {
	params           exporter.Settings
	cfg              *datadogconfig.Config
	agntConfig       *config.AgentConfig
	ctx              context.Context
	metricsAPI       *datadogV2.MetricsApi
	tr               otlpmetrics.Provider
	scrubber         scrub.Scrubber
	retrier          *clientutil.Retrier
	onceMetadata     *sync.Once
	sourceProvider   source.Provider
	metadataReporter *inframetadata.Reporter
	// getPushTime returns a Unix time in nanoseconds, representing the time pushing metrics.
	// It will be overwritten in tests.
	getPushTime func() uint64

	gatewayUsage *attributes.GatewayUsage
}

func newMetricsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg *datadogconfig.Config,
	agntConfig *config.AgentConfig,
	onceMetadata *sync.Once,
	attrsTranslator *attributes.Translator,
	sourceProvider source.Provider,
	metadataReporter *inframetadata.Reporter,
	statsOut chan []byte,
	gatewayUsage *attributes.GatewayUsage,
) (*metricsExporter, error) {
	options := cfg.Metrics.ToTranslatorOpts()
	options = append(options,
		otlpmetrics.WithFallbackSourceProvider(sourceProvider),
		otlpmetrics.WithStatsOut(statsOut))

	switch {
	case featuregates.DisableMetricRemappingFeatureGate.IsEnabled():
		options = append(options, otlpmetrics.WithoutRuntimeMetricMappings())
	case featuregates.MetricRemappingDisabledFeatureGate.IsEnabled():
		// Do nothing.
	default:
		options = append(options, otlpmetrics.WithRemapping())
	}

	if inferIntervalDeltaFeatureGate.IsEnabled() {
		params.Logger.Info("Metrics interval will be inferred for OTLP delta metrics with a set StartTimestamp.")
		options = append(options, otlpmetrics.WithInferDeltaInterval())
	}

	tr, err := otlpmetrics.NewDefaultTranslator(params.TelemetrySettings, attrsTranslator, options...)
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
		gatewayUsage:     gatewayUsage,
	}
	errchan := make(chan error)
	apiClient := clientutil.CreateAPIClient(
		params.BuildInfo,
		cfg.Metrics.Endpoint,
		cfg.ClientConfig)
	go func() { errchan <- clientutil.ValidateAPIKey(ctx, string(cfg.API.Key), params.Logger, apiClient) }()
	exporter.metricsAPI = datadogV2.NewMetricsApi(apiClient)
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
		exp.cfg.Metrics.Endpoint+sketches.SketchSeriesEndpoint,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return fmt.Errorf("failed to build sketches HTTP request: %w", err)
	}

	clientutil.SetDDHeaders(req.Header, exp.params.BuildInfo, string(exp.cfg.API.Key))
	clientutil.SetExtraHeaders(req.Header, clientutil.ProtobufHeaders)
	resp, err := exp.metricsAPI.Client.Cfg.HTTPClient.Do(req)
	if err != nil {
		return clientutil.WrapError(fmt.Errorf("failed to do sketches HTTP request: %w", err), resp)
	}
	defer resp.Body.Close()

	// We must read the full response body from the http request to ensure that connections can be
	// properly re-used. https://pkg.go.dev/net/http#Client.Do
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return clientutil.WrapError(fmt.Errorf("failed to read response body from sketches HTTP request: %w", err), resp)
	}

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
	consumer := metrics.NewConsumer(exp.gatewayUsage)
	metadata, err := exp.tr.MapMetrics(ctx, md, consumer, exp.gatewayUsage)
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
	var ms []datadogV2.MetricSeries
	ms, sl = consumer.All(exp.getPushTime(), exp.params.BuildInfo, tags, metadata)
	if len(ms) > 0 {
		exp.params.Logger.Debug("exporting native Datadog payload", zap.Any("metric", ms))
		_, experr := exp.retrier.DoWithRetries(ctx, func(context.Context) error {
			ctx = clientutil.GetRequestContext(ctx, string(exp.cfg.API.Key))
			_, httpresp, merr := exp.metricsAPI.SubmitMetrics(ctx, datadogV2.MetricPayload{Series: ms}, *clientutil.GZipSubmitMetricsOptionalParameters)
			return clientutil.WrapError(merr, httpresp)
		})
		errs = append(errs, experr)
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
