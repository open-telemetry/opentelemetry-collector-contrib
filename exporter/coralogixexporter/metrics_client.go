// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
	exp "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func newMetricsExporter(cfg component.Config, set exp.CreateSettings) (*exporter, error) {
	oCfg := cfg.(*Config)

	if isEmpty(oCfg.Domain) && isEmpty(oCfg.Metrics.Endpoint) {
		return nil, errors.New("coralogix exporter config requires `domain` or `metrics.endpoint` configuration")
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

func (e *exporter) start(ctx context.Context, host component.Host) (err error) {

	switch {
	case !isEmpty(e.config.Metrics.Endpoint):
		if e.clientConn, err = e.config.Metrics.ToClientConn(ctx, host, e.settings, grpc.WithUserAgent(e.userAgent)); err != nil {
			return err
		}
	case !isEmpty(e.config.Domain):
		if e.clientConn, err = e.config.getDomainGrpcSettings().ToClientConn(ctx, host, e.settings, grpc.WithUserAgent(e.userAgent)); err != nil {
			return err
		}
	}

	e.metricExporter = pmetricotlp.NewGRPCClient(e.clientConn)
	if e.config.Metrics.Headers == nil {
		e.config.Metrics.Headers = make(map[string]configopaque.String)
	}
	e.config.Metrics.Headers["Authorization"] = configopaque.String("Bearer " + string(e.config.PrivateKey))

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
		resourceMetric.Resource().Attributes().PutStr(cxAppNameAttrName, appName)
		resourceMetric.Resource().Attributes().PutStr(cxSubsystemNameAttrName, subsystem)
	}

	_, err := e.metricExporter.Export(e.enhanceContext(ctx), pmetricotlp.NewExportRequestFromMetrics(md), e.callOptions...)
	if err != nil {
		return processError(err)
	}

	return nil
}

func (e *exporter) shutdown(context.Context) error {
	if e.clientConn == nil {
		return nil
	}
	return e.clientConn.Close()
}

func (e *exporter) enhanceContext(ctx context.Context) context.Context {
	md := metadata.New(nil)
	for k, v := range e.config.Metrics.Headers {
		md.Set(k, string(v))
	}
	return metadata.NewOutgoingContext(ctx, md)
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
