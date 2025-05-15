// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Send a telemetry data request to the server. "perform" function is expected to make
// the actual gRPC unary call that sends the request. This function implements the
// common OTLP logic around request handling such as retries and throttling.
func processError(err error, signalExporter *signalExporter) error {
	if err == nil {
		return nil
	}

	st := status.Convert(err)
	if st.Code() == codes.OK {
		return nil
	}

	retryInfo := getRetryInfo(st)

	shouldRetry, shouldFlagRateLimit := shouldRetry(st.Code(), retryInfo)
	if !shouldRetry {
		if shouldFlagRateLimit {
			signalExporter.EnableRateLimit(err)
		}
		return consumererror.NewPermanent(err)
	}

	throttleDuration := getThrottleDuration(retryInfo)
	if throttleDuration != 0 {
		return exporterhelper.NewThrottleRetry(err, throttleDuration)
	}

	return err
}

// shouldRetry returns true if the error should be retried.
// The second return value indicates if the error should flag the rate limiting mechanism.
func shouldRetry(code codes.Code, retryInfo *errdetails.RetryInfo) (bool, bool) {
	switch code {
	case codes.Canceled,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unavailable,
		codes.DataLoss:
		return true, false
	case codes.ResourceExhausted:
		// Retry only if RetryInfo was supplied by the server.
		// This indicates that the server can still recover from resource exhaustion.
		return retryInfo != nil, retryInfo == nil
	case codes.Unauthenticated, codes.PermissionDenied:
		return false, true
	default:
		return false, false
	}
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
