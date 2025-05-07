// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"
)

type prwTelemetry interface {
	recordTranslationFailure(ctx context.Context)
	recordTranslatedTimeSeries(ctx context.Context, numTS int)
	recordRemoteWriteSentBatch(ctx context.Context)
	setNumberConsumer(ctx context.Context, n int64)
}

type prwTelemetryOtel struct {
	telemetryBuilder *metadata.TelemetryBuilder
	otelAttrs        []attribute.KeyValue
}

func (p *prwTelemetryOtel) setNumberConsumer(ctx context.Context, n int64) {
	p.telemetryBuilder.ExporterPrometheusremotewriteConsumers.Add(ctx, n, metric.WithAttributes(p.otelAttrs...))
}

func (p *prwTelemetryOtel) recordRemoteWriteSentBatch(ctx context.Context) {
	p.telemetryBuilder.ExporterPrometheusremotewriteSentBatches.Add(ctx, 1, metric.WithAttributes(p.otelAttrs...))
}

func (p *prwTelemetryOtel) recordTranslationFailure(ctx context.Context) {
	p.telemetryBuilder.ExporterPrometheusremotewriteFailedTranslations.Add(ctx, 1, metric.WithAttributes(p.otelAttrs...))
}

func (p *prwTelemetryOtel) recordTranslatedTimeSeries(ctx context.Context, numTS int) {
	p.telemetryBuilder.ExporterPrometheusremotewriteTranslatedTimeSeries.Add(ctx, int64(numTS), metric.WithAttributes(p.otelAttrs...))
}

type buffer struct {
	protobuf *proto.Buffer
	snappy   []byte
}

// A reusable buffer pool for serializing protobufs and compressing them with Snappy.
var bufferPool = sync.Pool{
	New: func() any {
		return &buffer{
			protobuf: proto.NewBuffer(nil),
			snappy:   nil,
		}
	},
}

// prwExporter converts OTLP metrics to Prometheus remote write TimeSeries and sends them to a remote endpoint.
type prwExporter struct {
	endpointURL         *url.URL
	client              *http.Client
	wg                  *sync.WaitGroup
	closeChan           chan struct{}
	concurrency         int
	userAgentHeader     string
	maxBatchSizeBytes   int
	clientSettings      *confighttp.ClientConfig
	settings            component.TelemetrySettings
	retrySettings       configretry.BackOffConfig
	retryOnHTTP429      bool
	wal                 *prweWAL
	exporterSettings    prometheusremotewrite.Settings
	telemetry           prwTelemetry
	RemoteWriteProtoMsg config.RemoteWriteProtoMsg

	// When concurrency is enabled, concurrent goroutines would potentially
	// fight over the same batchState object. To avoid this, we use a pool
	// to provide each goroutine with its own state.
	batchStatePool sync.Pool
}

func newPRWTelemetry(set exporter.Settings, endpointURL *url.URL) (prwTelemetry, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return &prwTelemetryOtel{
		telemetryBuilder: telemetryBuilder,
		otelAttrs: []attribute.KeyValue{
			attribute.String("exporter", set.ID.String()),
			attribute.String("endpoint", endpointURL.String()),
		},
	}, nil
}

// newPRWExporter initializes a new prwExporter instance and sets fields accordingly.
func newPRWExporter(cfg *Config, set exporter.Settings) (*prwExporter, error) {
	sanitizedLabels, err := validateAndSanitizeExternalLabels(cfg)
	if err != nil {
		return nil, err
	}

	endpointURL, err := url.ParseRequestURI(cfg.ClientConfig.Endpoint)
	if err != nil {
		return nil, errors.New("invalid endpoint")
	}

	telemetry, err := newPRWTelemetry(set, endpointURL)
	if err != nil {
		return nil, err
	}

	if err := config.RemoteWriteProtoMsg.Validate(cfg.RemoteWriteProtoMsg); err != nil {
		return nil, err
	}

	userAgentHeader := fmt.Sprintf("%s/%s", strings.ReplaceAll(strings.ToLower(set.BuildInfo.Description), " ", "-"), set.BuildInfo.Version)

	concurrency := 5
	if !enableMultipleWorkersFeatureGate.IsEnabled() {
		concurrency = cfg.RemoteWriteQueue.NumConsumers
	}
	if cfg.MaxBatchRequestParallelism != nil {
		concurrency = *cfg.MaxBatchRequestParallelism
	}

	// Set the desired number of consumers as a metric for the exporter.
	telemetry.setNumberConsumer(context.Background(), int64(concurrency))

	prwe := &prwExporter{
		endpointURL:         endpointURL,
		wg:                  new(sync.WaitGroup),
		closeChan:           make(chan struct{}),
		userAgentHeader:     userAgentHeader,
		maxBatchSizeBytes:   cfg.MaxBatchSizeBytes,
		concurrency:         cfg.RemoteWriteQueue.NumConsumers,
		clientSettings:      &cfg.ClientConfig,
		settings:            set.TelemetrySettings,
		retrySettings:       cfg.BackOffConfig,
		retryOnHTTP429:      retryOn429FeatureGate.IsEnabled(),
		RemoteWriteProtoMsg: cfg.RemoteWriteProtoMsg,
		exporterSettings: prometheusremotewrite.Settings{
			Namespace:         cfg.Namespace,
			ExternalLabels:    sanitizedLabels,
			DisableTargetInfo: !cfg.TargetInfo.Enabled,
			AddMetricSuffixes: cfg.AddMetricSuffixes,
			SendMetadata:      cfg.SendMetadata,
		},
		telemetry:      telemetry,
		batchStatePool: sync.Pool{New: func() any { return newBatchTimeServicesState() }},
	}

	prwe.settings.Logger.Info("starting prometheus remote write exporter", zap.Any("ProtoMsg", cfg.RemoteWriteProtoMsg))

	prwe.wal = newWAL(cfg.WAL, prwe.export)
	return prwe, nil
}

// Start creates the prometheus client
func (prwe *prwExporter) Start(ctx context.Context, host component.Host) (err error) {
	prwe.client, err = prwe.clientSettings.ToClient(ctx, host, prwe.settings)
	if err != nil {
		return err
	}
	return prwe.turnOnWALIfEnabled(contextWithLogger(ctx, prwe.settings.Logger.Named("prw.wal")))
}

func (prwe *prwExporter) shutdownWALIfEnabled() error {
	if !prwe.walEnabled() {
		return nil
	}
	return prwe.wal.stop()
}

// Shutdown stops the exporter from accepting incoming calls(and return error), and wait for current export operations
// to finish before returning
func (prwe *prwExporter) Shutdown(context.Context) error {
	select {
	case <-prwe.closeChan:
	default:
		close(prwe.closeChan)
	}
	err := prwe.shutdownWALIfEnabled()
	prwe.wg.Wait()
	return err
}

func (prwe *prwExporter) pushMetricsV1(ctx context.Context, md pmetric.Metrics) error {
	tsMap, err := prometheusremotewrite.FromMetrics(md, prwe.exporterSettings)

	prwe.telemetry.recordTranslatedTimeSeries(ctx, len(tsMap))

	var m []*prompb.MetricMetadata
	if prwe.exporterSettings.SendMetadata {
		m = prometheusremotewrite.OtelMetricsToMetadata(md, prwe.exporterSettings.AddMetricSuffixes)
	}
	if err != nil {
		prwe.telemetry.recordTranslationFailure(ctx)
		prwe.settings.Logger.Debug("failed to translate metrics, exporting remaining metrics", zap.Error(err), zap.Int("translated", len(tsMap)))
	}
	// Call export even if a conversion error, since there may be points that were successfully converted.
	return prwe.handleExport(ctx, tsMap, m)
}

// PushMetrics converts metrics to Prometheus remote write TimeSeries and send to remote endpoint. It maintain a map of
// TimeSeries, validates and handles each individual metric, adding the converted TimeSeries to the map, and finally
// exports the map.
func (prwe *prwExporter) PushMetrics(ctx context.Context, md pmetric.Metrics) error {
	prwe.wg.Add(1)
	defer prwe.wg.Done()

	select {
	case <-prwe.closeChan:
		return errors.New("shutdown has been called")
	default:

		// If feature flag not enabled support only RW1.
		if !enableSendingRW2FeatureGate.IsEnabled() {
			return prwe.pushMetricsV1(ctx, md)
		}

		// If feature flag was enabled check if we want to send RW1 or RW2.
		switch prwe.RemoteWriteProtoMsg {
		case config.RemoteWriteProtoMsgV1:
			return prwe.pushMetricsV1(ctx, md)
		case config.RemoteWriteProtoMsgV2:
			return prwe.pushMetricsV2(ctx, md)

		default:
			return fmt.Errorf("unsupported remote-write protobuf message: %v", prwe.RemoteWriteProtoMsg)
		}
	}
}

func validateAndSanitizeExternalLabels(cfg *Config) (map[string]string, error) {
	sanitizedLabels := make(map[string]string)
	for key, value := range cfg.ExternalLabels {
		if key == "" || value == "" {
			return nil, errors.New("prometheus remote write: external labels configuration contains an empty key or value")
		}
		sanitizedLabels[prometheustranslator.NormalizeLabel(key)] = value
	}

	return sanitizedLabels, nil
}

func (prwe *prwExporter) handleExport(ctx context.Context, tsMap map[string]*prompb.TimeSeries, m []*prompb.MetricMetadata) error {
	// There are no metrics to export, so return.
	if len(tsMap) == 0 {
		return nil
	}

	state := prwe.batchStatePool.Get().(*batchTimeSeriesState)
	defer prwe.batchStatePool.Put(state)
	// Calls the helper function to convert and batch the TsMap to the desired format
	requests, err := batchTimeSeries(tsMap, prwe.maxBatchSizeBytes, m, state)
	if err != nil {
		return err
	}
	if !prwe.walEnabled() {
		// Perform a direct export otherwise.
		return prwe.export(ctx, requests)
	}

	// Otherwise the WAL is enabled, and just persist the requests to the WAL
	// and they'll be exported in another goroutine to the RemoteWrite endpoint.
	if err = prwe.wal.persistToWAL(requests); err != nil {
		return consumererror.NewPermanent(err)
	}
	return nil
}

// export sends a Snappy-compressed WriteRequest containing TimeSeries to a remote write endpoint in order
func (prwe *prwExporter) export(ctx context.Context, requests []*prompb.WriteRequest) error {
	input := make(chan *prompb.WriteRequest, len(requests))
	for _, request := range requests {
		input <- request
	}
	close(input)

	var wg sync.WaitGroup

	concurrencyLimit := int(math.Min(float64(prwe.concurrency), float64(len(requests))))
	wg.Add(concurrencyLimit) // used to wait for workers to be finished

	var mu sync.Mutex
	var errs error
	// Run concurrencyLimit of workers until there
	// is no more requests to execute in the input channel.
	for i := 0; i < concurrencyLimit; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done(): // Check firstly to ensure that the context wasn't cancelled.
					return

				case request, ok := <-input:
					if !ok {
						return
					}

					buf := bufferPool.Get().(*buffer)
					buf.protobuf.Reset()
					defer bufferPool.Put(buf)

					if errMarshal := buf.protobuf.Marshal(request); errMarshal != nil {
						mu.Lock()
						errs = multierr.Append(errs, consumererror.NewPermanent(errMarshal))
						mu.Unlock()
						return
					}

					if errExecute := prwe.execute(ctx, buf); errExecute != nil {
						mu.Lock()
						errs = multierr.Append(errs, consumererror.NewPermanent(errExecute))
						mu.Unlock()
					}
				}
			}
		}()
	}
	wg.Wait()

	return errs
}

func (prwe *prwExporter) execute(ctx context.Context, buf *buffer) error {
	// If we don't pass a buffer large enough, Snappy Encode function will not use it and instead will allocate a new buffer.
	// Manually grow the buffer to make sure Snappy uses it and we can re-use it afterwards.
	maxCompressedLen := snappy.MaxEncodedLen(len(buf.protobuf.Bytes()))
	if maxCompressedLen > len(buf.snappy) {
		if cap(buf.snappy) < maxCompressedLen {
			buf.snappy = make([]byte, maxCompressedLen)
		} else {
			buf.snappy = buf.snappy[:maxCompressedLen]
		}
	}
	compressedData := snappy.Encode(buf.snappy, buf.protobuf.Bytes())

	// executeFunc can be used for backoff and non backoff scenarios.
	executeFunc := func() error {
		// check there was no timeout in the component level to avoid retries
		// to continue to run after a timeout
		select {
		case <-ctx.Done():
			return backoff.Permanent(ctx.Err())
		default:
			// continue
		}

		// Create the HTTP POST request to send to the endpoint
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, prwe.endpointURL.String(), bytes.NewReader(compressedData))
		if err != nil {
			return backoff.Permanent(consumererror.NewPermanent(err))
		}

		// Add necessary headers specified by:
		// https://cortexmetrics.io/docs/apis/#remote-api
		req.Header.Add("Content-Encoding", "snappy")
		req.Header.Set("User-Agent", prwe.userAgentHeader)

		switch {
		// If feature flag not enabled support only RW1
		case !enableSendingRW2FeatureGate.IsEnabled(), prwe.RemoteWriteProtoMsg == config.RemoteWriteProtoMsgV1:
			req.Header.Set("Content-Type", "application/x-protobuf")
			req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
		case prwe.RemoteWriteProtoMsg == config.RemoteWriteProtoMsgV2:
			req.Header.Set("Content-Type", "application/x-protobuf;proto=io.prometheus.write.v2.Request")
			req.Header.Set("X-Prometheus-Remote-Write-Version", "2.0.0")
		default:
			return fmt.Errorf("unsupported remote-write protobuf message: %v (should be validated earlier)", prwe.RemoteWriteProtoMsg)
		}

		resp, err := prwe.client.Do(req)
		prwe.telemetry.recordRemoteWriteSentBatch(ctx)
		if err != nil {
			return err
		}
		defer func() {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()

		// 2xx status code is considered a success
		// 5xx errors are recoverable and the exporter should retry
		// Reference for different behavior according to status code:
		// https://github.com/prometheus/prometheus/pull/2552/files#diff-ae8db9d16d8057358e49d694522e7186
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
		rerr := fmt.Errorf("remote write returned HTTP status %v; err = %w: %s", resp.Status, err, body)
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			return rerr
		}

		// 429 errors are recoverable and the exporter should retry if RetryOnHTTP429 enabled
		// Reference: https://github.com/prometheus/prometheus/pull/12677
		if prwe.retryOnHTTP429 && resp.StatusCode == http.StatusTooManyRequests {
			return rerr
		}

		return backoff.Permanent(consumererror.NewPermanent(rerr))
	}

	var err error
	if prwe.retrySettings.Enabled {
		// Use the BackOff instance to retry the func with exponential backoff.
		err = backoff.Retry(executeFunc, &backoff.ExponentialBackOff{
			InitialInterval:     prwe.retrySettings.InitialInterval,
			RandomizationFactor: prwe.retrySettings.RandomizationFactor,
			Multiplier:          prwe.retrySettings.Multiplier,
			MaxInterval:         prwe.retrySettings.MaxInterval,
			MaxElapsedTime:      prwe.retrySettings.MaxElapsedTime,
			Stop:                backoff.Stop,
			Clock:               backoff.SystemClock,
		})
	} else {
		err = executeFunc()
	}

	if err != nil {
		return consumererror.NewPermanent(err)
	}

	return err
}

func (prwe *prwExporter) walEnabled() bool { return prwe.wal != nil }

func (prwe *prwExporter) turnOnWALIfEnabled(ctx context.Context) error {
	if !prwe.walEnabled() {
		return nil
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	go func() {
		<-prwe.closeChan
		cancel()
	}()
	return prwe.wal.run(cancelCtx)
}
