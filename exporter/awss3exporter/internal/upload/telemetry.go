// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"

import (
	"context"
	"errors"
	"net"
	"syscall"
	"time"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadata"
)

const (
	attrOutcome   = "outcome"
	attrErrorType = "error.type"

	outcomeSuccess = "success"
	outcomeFailure = "failure"

	errorTypeTimeout           = "timeout"
	errorTypeCanceled          = "canceled"
	errorTypeDNS               = "dns"
	errorTypeConnectionRefused = "connection_refused"
	errorTypeNetwork           = "network"
	errorTypeOther             = "other"
)

var successAttributes = metric.WithAttributeSet(attribute.NewSet(attribute.String(attrOutcome, outcomeSuccess)))

// Telemetry records the exporter's metrics on top of the generated telemetry
// builder. Recording is skipped, without allocating, when the instruments are
// no-op because the collector has the metrics disabled. All methods are safe to
// call on a nil Telemetry, which records nothing.
type Telemetry struct {
	tb *metadata.TelemetryBuilder
}

func NewTelemetry(set component.TelemetrySettings) (*Telemetry, error) {
	tb, err := metadata.NewTelemetryBuilder(set)
	if err != nil {
		return nil, err
	}
	return &Telemetry{tb: tb}, nil
}

func (t *Telemetry) Shutdown() {
	if t != nil {
		t.tb.Shutdown()
	}
}

// StartUpload returns the start time of an upload, or the zero time when no
// upload instrument records. That spares timing the request.
func (t *Telemetry) StartUpload(ctx context.Context) time.Time {
	if t == nil || (!t.tb.Awss3UploadedSize.Enabled(ctx) && !t.tb.Awss3UploadDuration.Enabled(ctx)) {
		return time.Time{}
	}
	return time.Now()
}

// RecordUpload records an upload request of size bytes that started at start,
// as returned by StartUpload. It does nothing for the zero time.
func (t *Telemetry) RecordUpload(ctx context.Context, size int, start time.Time, err error) {
	if t == nil || start.IsZero() {
		return
	}
	if err == nil {
		t.tb.Awss3UploadedSize.Add(ctx, int64(size))
	}
	t.tb.Awss3UploadDuration.Record(ctx, time.Since(start).Seconds(), attributes(err))
}

// errorType classifies err into a value for the error.type attribute:
//   - timeout and canceled, for deadlines, timeouts and cancellations,
//   - the error code of any error response, as returned by the store (for
//     example AccessDenied or SlowDown for S3),
//   - dns, connection_refused and network, for failures to send the request,
//   - other, for the rest, such as failing to get credentials.
//
// The classification relies on the error types, never on error messages.
func errorType(err error) string {
	var (
		apiErr  smithy.APIError
		netErr  net.Error
		dnsErr  *net.DNSError
		sendErr *smithyhttp.RequestSendError
	)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return errorTypeTimeout
	case errors.Is(err, context.Canceled):
		return errorTypeCanceled
	case errors.As(err, &netErr) && netErr.Timeout():
		return errorTypeTimeout
	case errors.As(err, &apiErr) && apiErr.ErrorCode() != "":
		return apiErr.ErrorCode()
	case errors.As(err, &dnsErr):
		return errorTypeDNS
	case errors.Is(err, syscall.ECONNREFUSED):
		return errorTypeConnectionRefused
	case errors.As(err, &sendErr):
		// Any other failure to send the request, such as a connection reset or
		// a TLS failure, which the SDK does not type any further.
		return errorTypeNetwork
	default:
		return errorTypeOther
	}
}

func attributes(err error) metric.MeasurementOption {
	if err == nil {
		return successAttributes
	}
	return failureAttributes(errorType(err))
}

func failureAttributes(errType string) metric.MeasurementOption {
	return metric.WithAttributeSet(attribute.NewSet(
		attribute.String(attrOutcome, outcomeFailure),
		attribute.String(attrErrorType, errType),
	))
}
