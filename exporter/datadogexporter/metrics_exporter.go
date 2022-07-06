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

// nolint:gocritic
package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/service/featuregate"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/source"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/sketches"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils"
)

type metricsExporter struct {
	params         component.ExporterCreateSettings
	cfg            *config.Config
	ctx            context.Context
	client         *datadog.Client
	tr             *translator.Translator
	scrubber       scrub.Scrubber
	retrier        *utils.Retrier
	onceMetadata   *sync.Once
	sourceProvider source.Provider
}

// translatorFromConfig creates a new metrics translator from the exporter config.
func translatorFromConfig(logger *zap.Logger, cfg *config.Config, sourceProvider source.Provider) (*translator.Translator, error) {
	options := []translator.Option{
		translator.WithDeltaTTL(cfg.Metrics.DeltaTTL),
		translator.WithFallbackSourceProvider(sourceProvider),
	}

	if cfg.Metrics.HistConfig.SendCountSum {
		options = append(options, translator.WithCountSumMetrics())
	}

	switch cfg.Metrics.SummaryConfig.Mode {
	case config.SummaryModeGauges:
		options = append(options, translator.WithQuantiles())
	}

	if cfg.Metrics.ExporterConfig.ResourceAttributesAsTags {
		options = append(options, translator.WithResourceAttributesAsTags())
	}

	if cfg.Metrics.ExporterConfig.InstrumentationScopeMetadataAsTags && cfg.Metrics.ExporterConfig.InstrumentationLibraryMetadataAsTags { // nolint SA1019
		return nil, fmt.Errorf("cannot use both instrumentation_library_metadata_as_tags(deprecated) and instrumentation_scope_metadata_as_tags")
	}

	if cfg.Metrics.ExporterConfig.InstrumentationScopeMetadataAsTags {
		options = append(options, translator.WithInstrumentationScopeMetadataAsTags())
	}

	if cfg.Metrics.ExporterConfig.InstrumentationLibraryMetadataAsTags { // nolint SA1019
		options = append(options, translator.WithInstrumentationLibraryMetadataAsTags())
	}

	options = append(options, translator.WithHistogramMode(translator.HistogramMode(cfg.Metrics.HistConfig.Mode)))

	var numberMode translator.NumberMode
	switch cfg.Metrics.SumConfig.CumulativeMonotonicMode {
	case config.CumulativeMonotonicSumModeRawValue:
		numberMode = translator.NumberModeRawValue
	case config.CumulativeMonotonicSumModeToDelta:
		numberMode = translator.NumberModeCumulativeToDelta
	}

	options = append(options, translator.WithNumberMode(numberMode))

	if featuregate.GetRegistry().IsEnabled(metadata.HostnamePreviewFeatureGate) {
		options = append(options, translator.WithPreviewHostnameFromAttributes())
	}

	return translator.New(logger, options...)
}

func newMetricsExporter(ctx context.Context, params component.ExporterCreateSettings, cfg *config.Config, onceMetadata *sync.Once, sourceProvider source.Provider) (*metricsExporter, error) {
	client := utils.CreateClient(cfg.API.Key, cfg.Metrics.TCPAddr.Endpoint)
	client.ExtraHeader["User-Agent"] = utils.UserAgent(params.BuildInfo)
	client.HttpClient = utils.NewHTTPClient(cfg.TimeoutSettings, cfg.LimitedHTTPClientSettings.TLSSetting.InsecureSkipVerify)

	if err := utils.ValidateAPIKey(params.Logger, client); err != nil && cfg.API.FailOnInvalidKey {
		return nil, err
	}

	tr, err := translatorFromConfig(params.Logger, cfg, sourceProvider)
	if err != nil {
		return nil, err
	}

	scrubber := scrub.NewScrubber()
	return &metricsExporter{
		params:         params,
		cfg:            cfg,
		ctx:            ctx,
		client:         client,
		tr:             tr,
		scrubber:       scrubber,
		retrier:        utils.NewRetrier(params.Logger, cfg.RetrySettings, scrubber),
		onceMetadata:   onceMetadata,
		sourceProvider: sourceProvider,
	}, nil
}

func (exp *metricsExporter) pushSketches(ctx context.Context, sl sketches.SketchSeriesList) error {
	payload, err := sl.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal sketches: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		exp.cfg.Metrics.TCPAddr.Endpoint+sketches.SketchSeriesEndpoint,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return fmt.Errorf("failed to build sketches HTTP request: %w", err)
	}

	utils.SetDDHeaders(req.Header, exp.params.BuildInfo, exp.cfg.API.Key)
	utils.SetExtraHeaders(req.Header, utils.ProtobufHeaders)
	resp, err := exp.client.HttpClient.Do(req)

	if err != nil {
		return fmt.Errorf("failed to do sketches HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("error when sending payload to %s: %s", sketches.SketchSeriesEndpoint, resp.Status)
	}
	return nil
}

func (exp *metricsExporter) PushMetricsDataScrubbed(ctx context.Context, md pmetric.Metrics) error {
	return exp.scrubber.Scrub(exp.PushMetricsData(ctx, md))
}

func (exp *metricsExporter) PushMetricsData(ctx context.Context, md pmetric.Metrics) error {

	// Start host metadata with resource attributes from
	// the first payload.
	if exp.cfg.HostMetadata.Enabled {
		exp.onceMetadata.Do(func() {
			attrs := pcommon.NewMap()
			if md.ResourceMetrics().Len() > 0 {
				attrs = md.ResourceMetrics().At(0).Resource().Attributes()
			}
			go metadata.Pusher(exp.ctx, exp.params, newMetadataConfigfromConfig(exp.cfg), exp.sourceProvider, attrs)
		})
	}

	consumer := metrics.NewConsumer()
	pushTime := uint64(time.Now().UTC().UnixNano())
	err := exp.tr.MapMetrics(ctx, md, consumer)
	if err != nil {
		return fmt.Errorf("failed to map metrics: %w", err)
	}
	ms, sl := consumer.All(pushTime, exp.params.BuildInfo)
	metrics.ProcessMetrics(ms)

	err = nil
	if len(ms) > 0 {
		exp.params.Logger.Debug("exporting payload", zap.Any("metric", ms))
		err = multierr.Append(
			err,
			exp.retrier.DoWithRetries(ctx, func(context.Context) error {
				return exp.client.PostMetrics(ms)
			}),
		)
	}

	if len(sl) > 0 {
		exp.params.Logger.Debug("exporting sketches payload", zap.Any("sketches", sl))
		err = multierr.Append(
			err,
			exp.retrier.DoWithRetries(ctx, func(ctx context.Context) error {
				return exp.pushSketches(ctx, sl)
			}),
		)
	}

	return err
}
