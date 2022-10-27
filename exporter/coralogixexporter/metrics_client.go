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

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func newMetricsExporter(cfg config.Exporter, set component.ExporterCreateSettings) (*exporter, error) {
	oCfg := cfg.(*Config)

	if oCfg.Metrics.Endpoint == "" || oCfg.Metrics.Endpoint == "https://" || oCfg.Metrics.Endpoint == "http://" {
		return nil, errors.New("coralogix exporter config requires `metrics.endpoint` configuration")
	}
	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &exporter{config: oCfg, settings: set.TelemetrySettings, userAgent: userAgent}, nil
}

type exporter struct {
	// Input configuration.
	config *Config

	metricExporter pmetricotlp.GRPCClient
	clientConn     *grpc.ClientConn
	callOptions    []grpc.CallOption

	settings component.TelemetrySettings

	// Default user-agent header.
	userAgent string
}

func (e *exporter) start(_ context.Context, host component.Host) (err error) {
	dialOpts, err := e.config.Metrics.ToDialOptions(host, e.settings)
	if err != nil {
		return err
	}
	dialOpts = append(dialOpts, grpc.WithUserAgent(e.userAgent))

	if e.clientConn, err = grpc.Dial(e.config.Metrics.SanitizedEndpoint(), dialOpts...); err != nil {
		return err
	}

	e.metricExporter = pmetricotlp.NewGRPCClient(e.clientConn)
	if e.config.Metrics.Headers == nil {
		e.config.Metrics.Headers = make(map[string]string)
	}
	e.config.Metrics.Headers["Authorization"] = "Bearer " + e.config.PrivateKey

	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.Metrics.WaitForReady),
	}

	return
}

func (e *exporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {

	rss := md.ResourceMetrics()
	for i := 0; i < rss.Len(); i++ {
		resourceMetric := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceMetric.Resource())

		md := pmetric.NewMetrics()
		newRss := md.ResourceMetrics().AppendEmpty()
		resourceMetric.CopyTo(newRss)

		req := pmetricotlp.NewExportRequestFromMetrics(md)
		_, err := e.metricExporter.Export(e.enhanceContext(ctx, appName, subsystem), req, e.callOptions...)
		if err != nil {
			return processError(err)
		}
	}

	return nil
}

func (e *exporter) shutdown(context.Context) error {
	return e.clientConn.Close()
}

func (e *exporter) enhanceContext(ctx context.Context, appName, subSystemName string) context.Context {
	headers := make(map[string]string)
	for k, v := range e.config.Metrics.Headers {
		headers[k] = v
	}

	headers["ApplicationName"] = appName
	headers["ApiName"] = subSystemName

	return metadata.NewOutgoingContext(ctx, metadata.New(headers))
}

// Send a trace or metrics request to the server. "perform" function is expected to make
// the actual gRPC unary call that sends the request. This function implements the
// common OTLP logic around request handling such as retries and throttling.
func processError(err error) error {
	if err == nil {
		// Request is successful, we are done.
		return nil
	}

	// We have an error, check gRPC status code.

	st := status.Convert(err)
	if st.Code() == codes.OK {
		// Not really an error, still success.
		return nil
	}

	// Now, this is this a real error.

	retryInfo := getRetryInfo(st)

	if !shouldRetry(st.Code(), retryInfo) {
		// It is not a retryable error, we should not retry.
		return consumererror.NewPermanent(err)
	}

	// Check if server returned throttling information.
	throttleDuration := getThrottleDuration(retryInfo)
	if throttleDuration != 0 {
		// We are throttled. Wait before retrying as requested by the server.
		return exporterhelper.NewThrottleRetry(err, throttleDuration)
	}

	// Need to retry.

	return err
}

func shouldRetry(code codes.Code, retryInfo *errdetails.RetryInfo) bool {
	switch code {
	case codes.Canceled,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unavailable,
		codes.DataLoss:
		// These are retryable errors.
		return true
	case codes.ResourceExhausted:
		// Retry only if RetryInfo was supplied by the server.
		// This indicates that the server can still recover from resource exhaustion.
		return retryInfo != nil
	}
	// Don't retry on any other code.
	return false
}

func getRetryInfo(status *status.Status) *errdetails.RetryInfo {
	for _, detail := range status.Details() {
		if t, ok := detail.(*errdetails.RetryInfo); ok {
			return t
		}
	}
	return nil
}

func getThrottleDuration(t *errdetails.RetryInfo) time.Duration {
	if t == nil || t.RetryDelay == nil {
		return 0
	}
	if t.RetryDelay.Seconds > 0 || t.RetryDelay.Nanos > 0 {
		return time.Duration(t.RetryDelay.Seconds)*time.Second + time.Duration(t.RetryDelay.Nanos)*time.Nanosecond
	}
	return 0
}
