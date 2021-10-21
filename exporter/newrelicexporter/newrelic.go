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

package newrelicexporter

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	product = "NewRelic-OpenTelemetry-Collector"
)

// exporter exports OpenTelemetry Collector data to New Relic.
type exporter struct {
	buildInfo      *component.BuildInfo
	requestFactory telemetry.RequestFactory
	apiKeyHeader   string
	logger         *zap.Logger
}

type factoryBuilder func(options ...telemetry.ClientOption) (telemetry.RequestFactory, error)
type batchBuilder func() ([]telemetry.Batch, error)

func clientOptionForAPIKey(apiKey string) telemetry.ClientOption {
	if apiKey != "" {
		if len(apiKey) < 40 {
			return telemetry.WithInsertKey(apiKey)
		}
		return telemetry.WithLicenseKey(apiKey)
	}
	return nil
}

func clientOptions(info *component.BuildInfo, apiKey string, apiKeyHeader string, hostOverride string, insecure bool) []telemetry.ClientOption {
	userAgent := product
	if info.Version != "" {
		userAgent += "/" + info.Version
	}
	userAgent += " " + info.Command
	options := []telemetry.ClientOption{telemetry.WithUserAgent(userAgent)}
	if apiKey != "" {
		options = append(options, clientOptionForAPIKey(apiKey))
	} else if apiKeyHeader != "" {
		options = append(options, telemetry.WithNoDefaultKey())
	}

	if hostOverride != "" {
		options = append(options, telemetry.WithEndpoint(hostOverride))
	}

	if insecure {
		options = append(options, telemetry.WithInsecure())
	}
	return options
}

func newExporter(l *zap.Logger, buildInfo *component.BuildInfo, nrConfig EndpointConfig, createFactory factoryBuilder) (exporter, error) {
	options := clientOptions(
		buildInfo,
		nrConfig.APIKey,
		nrConfig.APIKeyHeader,
		nrConfig.HostOverride,
		nrConfig.insecure,
	)
	f, err := createFactory(options...)
	if nil != err {
		return exporter{}, err
	}
	return exporter{
		buildInfo:      buildInfo,
		requestFactory: f,
		apiKeyHeader:   strings.ToLower(nrConfig.APIKeyHeader),
		logger:         l,
	}, nil
}

func (e exporter) extractAPIKeyFromHeader(ctx context.Context) string {
	if e.apiKeyHeader == "" {
		return ""
	}

	// right now, we only support looking up attributes from requests that have gone through the gRPC server
	// in that case, it will add the HTTP headers as context metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// we have gRPC metadata in the context but does it have our key?
	values, ok := md[e.apiKeyHeader]
	if !ok {
		return ""
	}

	return values[0]
}

func (e exporter) pushTraceData(ctx context.Context, td pdata.Traces) (outputErr error) {
	details := newTraceMetadata(ctx)
	details.dataInputCount = td.SpanCount()
	builder := func() ([]telemetry.Batch, error) { return e.buildTraceBatch(&details, td) }
	return e.export(ctx, &details, builder)
}

func (e exporter) buildTraceBatch(details *exportMetadata, td pdata.Traces) ([]telemetry.Batch, error) {
	var errs error

	transform := newTransformer(e.logger, e.buildInfo, details)
	batches := make([]telemetry.Batch, 0, calcSpanBatches(td))

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.InstrumentationLibrarySpans().Len(); j++ {
			ispans := rspans.InstrumentationLibrarySpans().At(j)
			commonAttributes := transform.CommonAttributes(rspans.Resource(), ispans.InstrumentationLibrary())
			spanCommon, err := telemetry.NewSpanCommonBlock(telemetry.WithSpanAttributes(commonAttributes))
			if err != nil {
				e.logger.Error("Transform of span common attributes failed.", zap.Error(err))
				errs = multierr.Append(errs, err)
				continue
			}
			spans := make([]telemetry.Span, 0, ispans.Spans().Len())
			for k := 0; k < ispans.Spans().Len(); k++ {
				span := ispans.Spans().At(k)
				nrSpan, err := transform.Span(span)
				if err != nil {
					e.logger.Debug("Transform of span failed.", zap.Error(err))
					errs = multierr.Append(errs, err)
					continue
				}

				details.dataOutputCount++
				spans = append(spans, nrSpan)
			}
			batches = append(batches, telemetry.Batch{spanCommon, telemetry.NewSpanGroup(spans)})
		}
	}

	return batches, errs
}

func calcSpanBatches(td pdata.Traces) int {
	rss := td.ResourceSpans()
	batchCount := 0
	for i := 0; i < rss.Len(); i++ {
		batchCount += rss.At(i).InstrumentationLibrarySpans().Len()
	}
	return batchCount
}

func (e exporter) pushLogData(ctx context.Context, ld pdata.Logs) (outputErr error) {
	details := newLogMetadata(ctx)
	details.dataInputCount = ld.LogRecordCount()
	builder := func() ([]telemetry.Batch, error) { return e.buildLogBatch(&details, ld) }
	return e.export(ctx, &details, builder)
}

func (e exporter) buildLogBatch(details *exportMetadata, ld pdata.Logs) ([]telemetry.Batch, error) {
	var errs error

	transform := newTransformer(e.logger, e.buildInfo, details)
	batches := make([]telemetry.Batch, 0, calcLogBatches(ld))

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		for j := 0; j < rlogs.InstrumentationLibraryLogs().Len(); j++ {
			ilogs := rlogs.InstrumentationLibraryLogs().At(j)
			commonAttributes := transform.CommonAttributes(rlogs.Resource(), ilogs.InstrumentationLibrary())
			logCommon, err := telemetry.NewLogCommonBlock(telemetry.WithLogAttributes(commonAttributes))
			if err != nil {
				e.logger.Error("Transform of log common attributes failed.", zap.Error(err))
				errs = multierr.Append(errs, err)
				continue
			}
			logs := make([]telemetry.Log, 0, ilogs.Logs().Len())
			for k := 0; k < ilogs.Logs().Len(); k++ {
				log := ilogs.Logs().At(k)
				nrLog, err := transform.Log(log)
				if err != nil {
					e.logger.Error("Transform of log failed.", zap.Error(err))
					errs = multierr.Append(errs, err)
					continue
				}

				details.dataOutputCount++
				logs = append(logs, nrLog)
			}
			batches = append(batches, telemetry.Batch{logCommon, telemetry.NewLogGroup(logs)})
		}
	}

	return batches, errs
}

func calcLogBatches(ld pdata.Logs) int {
	rss := ld.ResourceLogs()
	batchCount := 0
	for i := 0; i < rss.Len(); i++ {
		batchCount += rss.At(i).InstrumentationLibraryLogs().Len()
	}
	return batchCount
}

func (e exporter) pushMetricData(ctx context.Context, md pdata.Metrics) (outputErr error) {
	details := newMetricMetadata(ctx)
	details.dataInputCount = md.DataPointCount()
	builder := func() ([]telemetry.Batch, error) { return e.buildMetricBatch(&details, md) }
	return e.export(ctx, &details, builder)
}

func (e exporter) buildMetricBatch(details *exportMetadata, md pdata.Metrics) ([]telemetry.Batch, error) {
	var errs error

	transform := newTransformer(e.logger, e.buildInfo, details)
	batches := make([]telemetry.Batch, 0, calcMetricBatches(md))

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.InstrumentationLibraryMetrics().Len(); j++ {
			imetrics := rmetrics.InstrumentationLibraryMetrics().At(j)
			commonAttributes := transform.CommonAttributes(rmetrics.Resource(), imetrics.InstrumentationLibrary())
			metricCommon, err := telemetry.NewMetricCommonBlock(telemetry.WithMetricAttributes(commonAttributes))
			if err != nil {
				e.logger.Error("Transform of metric common attributes failed.", zap.Error(err))
				errs = multierr.Append(errs, err)
				continue
			}
			metricSlices := make([][]telemetry.Metric, 0, imetrics.Metrics().Len())
			for k := 0; k < imetrics.Metrics().Len(); k++ {
				metric := imetrics.Metrics().At(k)
				nrMetrics, err := transform.Metric(metric)
				if err != nil {
					{
						var unsupportedErr *errUnsupportedMetricType
						if ok := errors.As(err, &unsupportedErr); ok {
							// Treat invalid metrics as a success
							details.dataOutputCount += unsupportedErr.numDataPoints
						}
					}
					e.logger.Debug("Transform of metric failed.", zap.Error(err))
					errs = multierr.Append(errs, err)
					continue
				}
				details.dataOutputCount += len(nrMetrics)
				metricSlices = append(metricSlices, nrMetrics)
			}
			metrics := combineMetricSlices(metricSlices)
			batches = append(batches, telemetry.Batch{metricCommon, telemetry.NewMetricGroup(metrics)})
		}
	}

	return batches, errs
}

func calcMetricBatches(md pdata.Metrics) int {
	rss := md.ResourceMetrics()
	batchCount := 0
	for i := 0; i < rss.Len(); i++ {
		batchCount += rss.At(i).InstrumentationLibraryMetrics().Len()
	}
	return batchCount
}

func combineMetricSlices(groups [][]telemetry.Metric) []telemetry.Metric {
	var totalLen int
	for _, group := range groups {
		totalLen += len(group)
	}
	metrics := make([]telemetry.Metric, totalLen)
	var i int
	for _, group := range groups {
		i += copy(metrics[i:], group)
	}
	return metrics
}

func (e exporter) export(
	ctx context.Context,
	details *exportMetadata,
	buildBatches batchBuilder,
) (outputErr error) {
	startTime := time.Now()
	apiKey := e.extractAPIKeyFromHeader(ctx)
	defer func() {
		details.apiKey = sanitizeAPIKeyForLogging(apiKey)
		details.exporterTime = time.Since(startTime)
		details.grpcResponseCode = status.Code(outputErr)
		err := details.recordMetrics(ctx)
		if err != nil {
			e.logger.Error("An error occurred recording metrics.", zap.Error(err))
		}
	}()

	batches, mapEntryErrors := buildBatches()

	var options []telemetry.ClientOption
	if option := clientOptionForAPIKey(apiKey); option != nil {
		options = append(options, option)
	}

	req, err := e.requestFactory.BuildRequest(ctx, batches, options...)
	if err != nil {
		e.logger.Error("Failed to build data map", zap.Error(err))
		return err
	}

	if err := e.doRequest(details, req); err != nil {
		return err
	}

	return mapEntryErrors
}

func (e exporter) doRequest(details *exportMetadata, req *http.Request) error {
	startTime := time.Now()
	defer func() { details.externalDuration = time.Since(startTime) }()

	// Execute the http request and handle the response
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		// If the context cancellation caused the error, we don't want to log that error.
		// Only log an error when the context is still active
		if ctxErr := req.Context().Err(); ctxErr == nil {
			e.logger.Error("Error making HTTP request.", zap.Error(err))
		}
		err := &urlError{err: err}
		return err.Wrap()
	}
	defer response.Body.Close()
	io.Copy(ioutil.Discard, response.Body)
	details.httpStatusCode = response.StatusCode

	// Check if the http payload has been accepted, if not record an error
	if response.StatusCode != http.StatusAccepted {
		// Log the error at an appropriate level based on the status code
		if response.StatusCode >= 500 {
			// The data has been lost, but it is due to a server side error
			e.logger.Warn("Server HTTP error", zap.String("Status", response.Status), zap.Stringer("Url", req.URL))
		} else if response.StatusCode == http.StatusForbidden {
			// The data has been lost, but it is due to an invalid api key
			e.logger.Debug("HTTP Forbidden response", zap.String("Status", response.Status), zap.Stringer("Url", req.URL))
		} else if response.StatusCode == http.StatusTooManyRequests {
			// The data has been lost, but it is due to rate limiting
			e.logger.Debug("HTTP Too Many Requests", zap.String("Status", response.Status), zap.Stringer("Url", req.URL))
		} else {
			// The data has been lost due to an error in our payload
			details.dataOutputCount = 0
			e.logger.Error("Client HTTP error.", zap.String("Status", response.Status), zap.Stringer("Url", req.URL))
		}

		err := newHTTPError(response)
		return err.Wrap()
	}
	return nil
}
