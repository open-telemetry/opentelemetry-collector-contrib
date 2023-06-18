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

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
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

// prwExporter converts OTLP metrics to Prometheus remote write TimeSeries and sends them to a remote endpoint.
type prwExporter struct {
	endpointURL      *url.URL
	client           *http.Client
	wg               *sync.WaitGroup
	closeChan        chan struct{}
	concurrency      int
	userAgentHeader  string
	maxBatchByteSize int
	clientSettings   *confighttp.HTTPClientSettings
	settings         component.TelemetrySettings

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
		endpointURL:      endpointURL,
		wg:               new(sync.WaitGroup),
		closeChan:        make(chan struct{}),
		userAgentHeader:  userAgentHeader,
		maxBatchByteSize: cfg.maxBatchByteSize,
		concurrency:      cfg.RemoteWriteQueue.NumConsumers,
		clientSettings:   &cfg.HTTPClientSettings,
		settings:         set.TelemetrySettings,
		exporterSettings: prometheusremotewrite.Settings{
			Namespace:           cfg.Namespace,
			ExternalLabels:      sanitizedLabels,
			DisableTargetInfo:   !cfg.TargetInfo.Enabled,
			ExportCreatedMetric: cfg.CreatedMetric.Enabled,
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
	prwe.client, err = prwe.clientSettings.ToClient(host, prwe.settings)
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

func (prwe *prwExporter) handleExport(ctx context.Context, tsMap map[string]*prompb.TimeSeries) error {
	// There are no metrics to export, so return.
	if len(tsMap) == 0 {
		return nil
	}

	// Calls the helper function to convert and batch the TsMap to the desired format
	requests, err := batchTimeSeries(tsMap, prwe.maxBatchByteSize)
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
					if errExecute := prwe.execute(ctx, request); errExecute != nil {
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
