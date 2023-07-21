// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"
)

const maxBatchByteSize = 3000000

// prwExporter converts OTLP metrics to Prometheus remote write TimeSeries and sends them to a remote endpoint.
type prwExporter struct {
	endpointURL     *url.URL
	client          *http.Client
	wg              *sync.WaitGroup
	closeChan       chan struct{}
	concurrency     int
	userAgentHeader string
	clientSettings  *confighttp.HTTPClientSettings
	settings        component.TelemetrySettings

	wal              *prweWAL
	exporterSettings prometheusremotewrite.Settings
}

// newPRWExporter initializes a new prwExporter instance and sets fields accordingly.
func newPRWExporter(cfg *Config, set exporter.CreateSettings) (*prwExporter, error) {
	sanitizedLabels, err := validateAndSanitizeExternalLabels(cfg)
	if err != nil {
		return nil, err
	}

	endpointURL, err := url.ParseRequestURI(cfg.HTTPClientSettings.Endpoint)
	if err != nil {
		return nil, errors.New("invalid endpoint")
	}

	userAgentHeader := fmt.Sprintf("%s/%s", strings.ReplaceAll(strings.ToLower(set.BuildInfo.Description), " ", "-"), set.BuildInfo.Version)

	prwe := &prwExporter{
		endpointURL:     endpointURL,
		wg:              new(sync.WaitGroup),
		closeChan:       make(chan struct{}),
		userAgentHeader: userAgentHeader,
		concurrency:     cfg.RemoteWriteQueue.NumConsumers,
		clientSettings:  &cfg.HTTPClientSettings,
		settings:        set.TelemetrySettings,
		exporterSettings: prometheusremotewrite.Settings{
			Namespace:           cfg.Namespace,
			ExternalLabels:      sanitizedLabels,
			DisableTargetInfo:   !cfg.TargetInfo.Enabled,
			ExportCreatedMetric: cfg.CreatedMetric.Enabled,
			AddMetricSuffixes:   cfg.AddMetricSuffixes,
		},
	}
	if cfg.WAL == nil {
		return prwe, nil
	}

	prwe.wal, err = newWAL(cfg.WAL, prwe.export)
	if err != nil {
		return nil, err
	}
	return prwe, nil
}

// Start creates the prometheus client
func (prwe *prwExporter) Start(ctx context.Context, host component.Host) (err error) {
	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient, err = prwe.clientSettings.ToClient(host, prwe.settings)
	if err != nil {
		return err
	}

	// Configure retry settings
	// Default settings reference - https://github.com/hashicorp/go-retryablehttp/blob/571a88bc9c3b7c64575f0e9b0f646af1510f2c76/client.go#L51-L53
	retryClient.RetryWaitMin = 100 * time.Millisecond
	retryClient.RetryWaitMax = 1 * time.Second
	retryClient.RetryMax = 3

	prwe.client = retryClient.HTTPClient
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
		tsMap, err := prometheusremotewrite.FromMetrics(md, prwe.exporterSettings)
		if err != nil {
			err = consumererror.NewPermanent(err)
		}
		// Call export even if a conversion error, since there may be points that were successfully converted.
		return multierr.Combine(err, prwe.handleExport(ctx, tsMap))
	}
}

func validateAndSanitizeExternalLabels(cfg *Config) (map[string]string, error) {
	sanitizedLabels := make(map[string]string)
	for key, value := range cfg.ExternalLabels {
		if key == "" || value == "" {
			return nil, fmt.Errorf("prometheus remote write: external labels configuration contains an empty key or value")
		}
		sanitizedLabels[prometheustranslator.NormalizeLabel(key)] = value
	}

	return sanitizedLabels, nil
}

// labelsToString converts labels to a string representation.
func labelsToString(labels []prompb.Label) string {
	var sb strings.Builder
	for _, l := range labels {
		sb.WriteString(l.Name)
		sb.WriteByte('=')
		sb.WriteString(l.Value)
		sb.WriteByte(',')
	}
	return sb.String()
}

// handleExport partitions the time series map into N arrays and creates N workers to process them.
func (prwe *prwExporter) handleExport(ctx context.Context, tsMap map[string]*prompb.TimeSeries) error {
	// There are no metrics to export, so return.
	if len(tsMap) == 0 {
		return nil
	}

	// Determine the number of partitions (N) based on the concurrency limit and the number of requests
	concurrencyLimit := int(math.Min(float64(prwe.concurrency), float64(len(tsMap))))

	// Partition the time series map into N arrays
	partitions := partitionTimeSeries(tsMap, concurrencyLimit)

	// Create N workers
	wg := sync.WaitGroup{}
	wg.Add(concurrencyLimit)

	for i := 0; i < concurrencyLimit; i++ {
		// Submit one array per worker
		go func(partition map[string]*prompb.TimeSeries) {
			defer wg.Done()

			requests, err := batchTimeSeries(partition, maxBatchByteSize)
			if err != nil {
				err = consumererror.NewPermanent(err)
			}

			if !prwe.walEnabled() {
				// WAL is not enabled, perform a direct export
				if errExecute := prwe.export(ctx, requests); errExecute != nil {
					// We will log the error here, but not return it immediately.
					// Instead, we'll let the worker complete its execution.
					prwe.settings.Logger.Error("Failed to export data:", zap.Error(errExecute))
				}
			} else {
				// WAL is enabled, persist requests to the WAL for later export
				if err := prwe.wal.persistToWAL(requests); err != nil {
					// Log the error and return a permanent consumer error
					err = consumererror.NewPermanent(err)
					prwe.settings.Logger.Error("Failed to persist to WAL:", zap.Error(err))
				}
			}
		}(partitions[i])
	}

	// Wait for all workers to finish processing the requests
	wg.Wait()

	return nil
}

// partitionTimeSeries partitions the time series map into N arrays.
func partitionTimeSeries(tsMap map[string]*prompb.TimeSeries, concurrencyLimit int) []map[string]*prompb.TimeSeries {
	partitions := make([]map[string]*prompb.TimeSeries, concurrencyLimit)
	i := 0
	for _, ts := range tsMap {
		if partitions[i] == nil {
			partitions[i] = make(map[string]*prompb.TimeSeries)
		}
		partitions[i][labelsToString(ts.Labels)] = ts
		i++
		if i >= concurrencyLimit {
			i = 0
		}
	}
	return partitions
}

func (prwe *prwExporter) export(ctx context.Context, requests []*prompb.WriteRequest) error {
	// Use a wait group to wait for the goroutine to finish processing the requests
	var wg sync.WaitGroup
	wg.Add(len(requests))

	// Use a mutex to handle concurrent errors
	var mu sync.Mutex
	var errs error

	// Run the goroutine to process each WriteRequest in the slice
	for _, request := range requests {
		go func(request *prompb.WriteRequest) {
			defer wg.Done()

			// Check if the context is already cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Process the WriteRequest
			if errExecute := prwe.execute(ctx, request); errExecute != nil {
				mu.Lock()
				errs = multierr.Append(errs, consumererror.NewPermanent(errExecute))
				mu.Unlock()
			}
		}(request)
	}

	// Wait for all goroutines to finish processing the requests
	wg.Wait()

	return errs
}

func (prwe *prwExporter) execute(ctx context.Context, writeReq *prompb.WriteRequest) error {
	// Uses proto.Marshal to convert the WriteRequest into bytes array
	data, err := proto.Marshal(writeReq)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	buf := make([]byte, len(data), cap(data))
	compressedData := snappy.Encode(buf, data)

	// Create the HTTP POST request to send to the endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", prwe.endpointURL.String(), bytes.NewReader(compressedData))
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	// Add necessary headers specified by:
	// https://cortexmetrics.io/docs/apis/#remote-api
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Set("User-Agent", prwe.userAgentHeader)

	resp, err := prwe.client.Do(req)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	defer resp.Body.Close()

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
	return consumererror.NewPermanent(rerr)
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
