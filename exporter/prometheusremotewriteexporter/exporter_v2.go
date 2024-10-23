// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"bytes"
	"context"
	"fmt"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"io"
	"math"
	"net/http"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/multierr"
)

func (prwe *prwExporter) handleExportV2(ctx context.Context, tsMap map[string]*writev2.TimeSeries, symbolsTable writev2.SymbolsTable) error {
	// There are no metrics to export, so return.
	if len(tsMap) == 0 {
		return nil
	}

	// Calls the helper function to convert and batch the TsMap to the desired format
	requests, err := batchTimeSeriesV2(tsMap, symbolsTable, prwe.maxBatchSizeBytes, &prwe.batchTimeSeriesState)
	if err != nil {
		return err
	}

	// TODO implement WAl support, can be done after #15277 is fixed

	return prwe.exportV2(ctx, requests)
}

// TODO update comment
// export sends a Snappy-compressed WriteRequest containing TimeSeries to a remote write endpoint in order
func (prwe *prwExporter) exportV2(ctx context.Context, requests []*writev2.Request) error {
	input := make(chan *writev2.Request, len(requests))
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
					if errExecute := prwe.executeV2(ctx, request); errExecute != nil {
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

func (prwe *prwExporter) executeV2(ctx context.Context, writeReq *writev2.Request) error {
	// Uses proto.Marshal to convert the WriteRequest into bytes array
	data, errMarshal := proto.Marshal(writeReq)
	if errMarshal != nil {
		return consumererror.NewPermanent(errMarshal)
	}
	// If we don't pass a buffer large enough, Snappy Encode function will not use it and instead will allocate a new buffer.
	// Therefore we always let Snappy decide the size of the buffer.
	compressedData := snappy.Encode(nil, data)

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
		req, err := http.NewRequestWithContext(ctx, "POST", prwe.endpointURL.String(), bytes.NewReader(compressedData))
		if err != nil {
			return backoff.Permanent(consumererror.NewPermanent(err))
		}

		// Add necessary headers specified by:
		// https://cortexmetrics.io/docs/apis/#remote-api
		req.Header.Add("Content-Encoding", "snappy")
		req.Header.Set("Content-Type", "application/x-protobuf;proto=io.prometheus.write.v2.Request")
		req.Header.Set("X-Prometheus-Remote-Write-Version", "2.0.0")
		req.Header.Set("User-Agent", prwe.userAgentHeader)

		resp, err := prwe.client.Do(req)
		if err != nil {
			return err
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

		// 429 errors are recoverable and the exporter should retry if RetryOnHTTP429 enabled
		// Reference: https://github.com/prometheus/prometheus/pull/12677
		if prwe.retryOnHTTP429 && resp.StatusCode == 429 {
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
