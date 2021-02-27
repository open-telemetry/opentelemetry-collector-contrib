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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/golang/protobuf/ptypes/duration"
	"github.com/newrelic/newrelic-telemetry-sdk-go/cumulative"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	name    = "opentelemetry-collector"
	version = "0.0.0"
	product = "NewRelic-Collector-OpenTelemetry"
)

var _ io.Writer = logWriter{}

// logWriter wraps a zap.Logger into an io.Writer.
type logWriter struct {
	logf func(string, ...zapcore.Field)
}

// Write implements io.Writer
func (w logWriter) Write(p []byte) (n int, err error) {
	w.logf(string(p))
	return len(p), nil
}

// exporter exporters OpenTelemetry Collector data to New Relic.
type exporter struct {
	deltaCalculator    *cumulative.DeltaCalculator
	harvester          *telemetry.Harvester
	spanRequestFactory telemetry.RequestFactory
	apiKeyHeader       string
	logger             *zap.Logger
}

func newMetricsExporter(l *zap.Logger, c configmodels.Exporter) (*exporter, error) {
	nrConfig, ok := c.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", c)
	}

	opts := []func(*telemetry.Config){
		nrConfig.HarvestOption,
		telemetry.ConfigBasicErrorLogger(logWriter{l.Error}),
		telemetry.ConfigBasicDebugLogger(logWriter{l.Info}),
		telemetry.ConfigBasicAuditLogger(logWriter{l.Debug}),
	}

	h, err := telemetry.NewHarvester(opts...)
	if nil != err {
		return nil, err
	}

	return &exporter{
		deltaCalculator: cumulative.NewDeltaCalculator(),
		harvester:       h,
	}, nil
}

func newTraceExporter(l *zap.Logger, c configmodels.Exporter) (*exporter, error) {
	nrConfig, ok := c.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", c)
	}

	options := []telemetry.ClientOption{telemetry.WithUserAgent(product + "/" + version)}
	if nrConfig.APIKey != "" {
		options = append(options, telemetry.WithInsertKey(nrConfig.APIKey))
	} else if nrConfig.APIKeyHeader != "" {
		options = append(options, telemetry.WithNoDefaultKey())
	}

	// TODO: Need to make sure this supports a URL with a scheme and path
	if nrConfig.SpansURLOverride != "" {
		options = append(options, telemetry.WithEndpoint(nrConfig.SpansURLOverride))
	}
	s, err := telemetry.NewSpanRequestFactory(options...)
	if nil != err {
		return nil, err
	}

	return &exporter{
		spanRequestFactory: s,
		apiKeyHeader:       strings.ToLower(nrConfig.APIKeyHeader),
		logger:             l,
	}, nil
}

func (e *exporter) extractInsertKeyFromHeader(ctx context.Context) string {
	if "" == e.apiKeyHeader {
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

func (e exporter) pushTraceData(ctx context.Context, td pdata.Traces) (int, error) {
	var (
		errs      []error
		goodSpans int
	)

	var batch telemetry.SpanBatch

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		resource := rspans.Resource()
		for j := 0; j < rspans.InstrumentationLibrarySpans().Len(); j++ {
			ispans := rspans.InstrumentationLibrarySpans().At(j)
			transform := newTraceTransformer(resource, ispans.InstrumentationLibrary())
			spans := make([]telemetry.Span, 0, ispans.Spans().Len())
			for k := 0; k < ispans.Spans().Len(); k++ {
				span := ispans.Spans().At(k)
				nrSpan, err := transform.Span(span)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				spans = append(spans, nrSpan)
				goodSpans++
			}
			batch.Spans = append(batch.Spans, spans...)
		}
	}
	batches := []telemetry.PayloadEntry{&batch}
	insertKey := e.extractInsertKeyFromHeader(ctx)
	var req *http.Request
	var err error

	if "" != insertKey {
		req, err = e.spanRequestFactory.BuildRequest(batches, telemetry.WithInsertKey(insertKey))
	} else {
		req, err = e.spanRequestFactory.BuildRequest(batches)
	}
	if err != nil {
		e.logger.Error("Failed to build batch", zap.Error(err))
		return 0, err
	}

	// Execute the http request and handle the response
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, e.handleHTTPError(err)
	}
	defer response.Body.Close()
	io.Copy(ioutil.Discard, response.Body)

	if response.StatusCode == http.StatusAccepted || response.StatusCode == http.StatusOK {
		return td.SpanCount() - goodSpans, componenterror.CombineErrors(errs)
	}
	return 0, e.handleResponseError(response)
}

func (e exporter) handleHTTPError(err error) error {
	e.logger.Error("Error making HTTP request.", zap.Error(err))
	urlError := err.(*url.Error)
	// If error is temporary, return retryable DataLoss code
	if urlError.Temporary() {
		return status.Errorf(codes.DataLoss, urlError.Error())
	}
	// Else, return non-retryable Internal code
	return status.Errorf(codes.Internal, urlError.Error())
}

// Explicit mapping for the error status codes describe by the trace API:
// https://docs.newrelic.com/docs/understand-dependencies/distributed-tracing/trace-api/trace-api-general-requirements-limits#response-validation
var httpGrpcMapping = map[int]codes.Code{
	http.StatusBadRequest:                  codes.InvalidArgument,
	http.StatusForbidden:                   codes.Unauthenticated,
	http.StatusNotFound:                    codes.NotFound,
	http.StatusMethodNotAllowed:            codes.InvalidArgument,
	http.StatusRequestTimeout:              codes.DeadlineExceeded,
	http.StatusLengthRequired:              codes.InvalidArgument,
	http.StatusRequestEntityTooLarge:       codes.InvalidArgument,
	http.StatusRequestURITooLong:           codes.InvalidArgument,
	http.StatusUnsupportedMediaType:        codes.InvalidArgument,
	http.StatusTooManyRequests:             codes.Unavailable,
	http.StatusRequestHeaderFieldsTooLarge: codes.InvalidArgument,
	http.StatusInternalServerError:         codes.DataLoss,
}

func (e exporter) handleResponseError(response *http.Response) error {
	// Log the error at an appropriate level based on the status code
	if response.StatusCode >= 500 {
		e.logger.Error("Error on HTTP response.", zap.String("Status", response.Status))
	} else {
		e.logger.Debug("Error on HTTP response.", zap.String("Status", response.Status))
	}

	mapEntry, ok := httpGrpcMapping[response.StatusCode]
	// If no explicit mapping exists, return retryable DataLoss code
	if !ok {
		return status.Errorf(codes.DataLoss, response.Status)
	}
	// The OTLP spec uses the Unavailable code to signal backpressure to the client
	// If the http status maps to Unavailable, attempt to extract and communicate retry info to the client
	if mapEntry == codes.Unavailable {
		retryAfter := response.Header.Get("Retry-After")
		retrySeconds, err := strconv.ParseInt(retryAfter, 10, 64)
		if err == nil {
			message := &errdetails.RetryInfo{RetryDelay: &duration.Duration{Seconds: retrySeconds}}
			status, statusErr := status.New(codes.Unavailable, response.Status).WithDetails(message)
			if statusErr == nil {
				return status.Err()
			}
		}
	}

	// Generate an error with the mapped code, and a message containing the server's response status string
	return status.Errorf(mapEntry, response.Status)
}

func (e exporter) pushMetricData(ctx context.Context, md pdata.Metrics) (int, error) {
	var errs []error
	goodMetrics := 0

	ocmds := internaldata.MetricsToOC(md)
	for _, ocmd := range ocmds {
		var srv string
		if ocmd.Node != nil && ocmd.Node.ServiceInfo != nil {
			srv = ocmd.Node.ServiceInfo.Name
		}

		transform := &metricTransformer{
			DeltaCalculator: e.deltaCalculator,
			ServiceName:     srv,
			Resource:        ocmd.Resource,
		}

		for _, metric := range ocmd.Metrics {
			nrMetrics, err := transform.Metric(metric)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			// TODO: optimize this, RecordMetric locks each call.
			for _, m := range nrMetrics {
				e.harvester.RecordMetric(m)
			}
			goodMetrics++
		}
	}

	e.harvester.HarvestNow(ctx)

	return md.MetricCount() - goodMetrics, componenterror.CombineErrors(errs)
}

func (e exporter) Shutdown(ctx context.Context) error {
	e.harvester.HarvestNow(ctx)
	return nil
}
