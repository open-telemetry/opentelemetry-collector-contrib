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
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/otlp/model/source"
	"github.com/DataDog/datadog-agent/pkg/otlp/model/translator"
	"github.com/DataDog/datadog-agent/pkg/trace/api"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	zorkian "gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics/sketches"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
)

type metricsExporter struct {
	params         exporter.CreateSettings
	cfg            *Config
	ctx            context.Context
	client         *zorkian.Client
	tr             *translator.Translator
	scrubber       scrub.Scrubber
	retrier        *clientutil.Retrier
	onceMetadata   *sync.Once
	sourceProvider source.Provider
	// getPushTime returns a Unix time in nanoseconds, representing the time pushing metrics.
	// It will be overwritten in tests.
	getPushTime       func() uint64
	apmStatsProcessor api.StatsProcessor
}

// translatorFromConfig creates a new metrics translator from the exporter
func translatorFromConfig(logger *zap.Logger, cfg *Config, sourceProvider source.Provider) (*translator.Translator, error) {
	options := []translator.Option{
		translator.WithDeltaTTL(cfg.Metrics.DeltaTTL),
		translator.WithFallbackSourceProvider(sourceProvider),
	}

	if cfg.Metrics.HistConfig.SendCountSum {
		options = append(options, translator.WithCountSumMetrics())
	}

	if cfg.Metrics.SummaryConfig.Mode == SummaryModeGauges {
		options = append(options, translator.WithQuantiles())
	}

	if cfg.Metrics.ExporterConfig.ResourceAttributesAsTags {
		options = append(options, translator.WithResourceAttributesAsTags())
	}

	if cfg.Metrics.ExporterConfig.InstrumentationScopeMetadataAsTags {
		options = append(options, translator.WithInstrumentationScopeMetadataAsTags())
	}

	options = append(options, translator.WithHistogramMode(translator.HistogramMode(cfg.Metrics.HistConfig.Mode)))

	var numberMode translator.NumberMode
	switch cfg.Metrics.SumConfig.CumulativeMonotonicMode {
	case CumulativeMonotonicSumModeRawValue:
		numberMode = translator.NumberModeRawValue
	case CumulativeMonotonicSumModeToDelta:
		numberMode = translator.NumberModeCumulativeToDelta
	}

	options = append(options, translator.WithNumberMode(numberMode))

	if featuregate.GetRegistry().IsEnabled(metadata.HostnamePreviewFeatureGate) {
		options = append(options, translator.WithPreviewHostnameFromAttributes())
	}

	return translator.New(logger, options...)
}

func newMetricsExporter(ctx context.Context, params exporter.CreateSettings, cfg *Config, onceMetadata *sync.Once, sourceProvider source.Provider, apmStatsProcessor api.StatsProcessor) (*metricsExporter, error) {
	client := clientutil.CreateZorkianClient(cfg.API.Key, cfg.Metrics.TCPAddr.Endpoint)
	client.ExtraHeader["User-Agent"] = clientutil.UserAgent(params.BuildInfo)
	client.HttpClient = clientutil.NewHTTPClient(cfg.TimeoutSettings, cfg.LimitedHTTPClientSettings.TLSSetting.InsecureSkipVerify)

	if err := clientutil.ValidateAPIKeyZorkian(params.Logger, client); err != nil && cfg.API.FailOnInvalidKey {
		return nil, err
	}

	tr, err := translatorFromConfig(params.Logger, cfg, sourceProvider)
	if err != nil {
		return nil, err
	}

	scrubber := scrub.NewScrubber()
	return &metricsExporter{
		params:            params,
		cfg:               cfg,
		ctx:               ctx,
		client:            client,
		tr:                tr,
		scrubber:          scrubber,
		retrier:           clientutil.NewRetrier(params.Logger, cfg.RetrySettings, scrubber),
		onceMetadata:      onceMetadata,
		sourceProvider:    sourceProvider,
		getPushTime:       func() uint64 { return uint64(time.Now().UTC().UnixNano()) },
		apmStatsProcessor: apmStatsProcessor,
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

	clientutil.SetDDHeaders(req.Header, exp.params.BuildInfo, exp.cfg.API.Key)
	clientutil.SetExtraHeaders(req.Header, clientutil.ProtobufHeaders)
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
	consumer := metrics.NewZorkianConsumer()
	err := exp.tr.MapMetrics(ctx, md, consumer)
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
	ms, sl, sp := consumer.All(exp.getPushTime(), exp.params.BuildInfo, tags)
	ms = metrics.PrepareZorkianSystemMetrics(ms)

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

	if len(sp) > 0 {
		exp.params.Logger.Debug("exporting APM stats payloads", zap.Any("stats_payloads", sp))
		statsv := exp.params.BuildInfo.Command + exp.params.BuildInfo.Version
		for _, p := range sp {
			exp.apmStatsProcessor.ProcessStats(p, "", statsv)
		}
	}

	return err
}
