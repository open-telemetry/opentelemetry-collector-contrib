// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/otel/attribute"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadatatest"
)

func newTestTelemetry(t *testing.T) (*Telemetry, *componenttest.Telemetry) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { assert.NoError(t, tt.Shutdown(context.WithoutCancel(t.Context()))) })
	tel, err := NewTelemetry(metadatatest.NewSettings(tt).TelemetrySettings)
	require.NoError(t, err)
	t.Cleanup(tel.Shutdown)
	return tel, tt
}

func successSet() attribute.Set {
	return attribute.NewSet(attribute.String("outcome", "success"))
}

func failureSet(errorType string) attribute.Set {
	return attribute.NewSet(attribute.String("outcome", "failure"), attribute.String("error.type", errorType))
}

// assertLatency checks the latency series only, as the measured durations are not deterministic.
func assertLatency(t *testing.T, tt *componenttest.Telemetry, want ...attribute.Set) {
	t.Helper()
	dps := make([]metricdata.HistogramDataPoint[float64], 0, len(want))
	for _, attrs := range want {
		dps = append(dps, metricdata.HistogramDataPoint[float64]{Attributes: attrs})
	}
	metadatatest.AssertEqualAwss3UploadDuration(t, tt, dps, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return false }

func TestErrorType(t *testing.T) {
	apiError := &smithy.GenericAPIError{Code: "AccessDenied", Message: "m"}
	refused := &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	dns := &net.DNSError{Err: "no such host", Name: "bucket.example", IsNotFound: true}

	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"deadline":                  {context.DeadlineExceeded, "timeout"},
		"wrapped deadline":          {fmt.Errorf("upload: %w", context.DeadlineExceeded), "timeout"},
		"canceled":                  {context.Canceled, "canceled"},
		"wrapped canceled":          {fmt.Errorf("upload: %w", context.Canceled), "canceled"},
		"network timeout":           {&smithyhttp.RequestSendError{Err: timeoutError{}}, "timeout"},
		"api error":                 {apiError, "AccessDenied"},
		"wrapped api error":         {&smithy.OperationError{ServiceID: "S3", OperationName: "PutObject", Err: apiError}, "AccessDenied"},
		"api error without code":    {&smithy.GenericAPIError{}, "other"},
		"dns":                       {dns, "dns"},
		"dns on send":               {&smithyhttp.RequestSendError{Err: dns}, "dns"},
		"connection refused":        {refused, "connection_refused"},
		"connection refused (send)": {&smithyhttp.RequestSendError{Err: refused}, "connection_refused"},
		"other send failure":        {&smithyhttp.RequestSendError{Err: errors.New("connection reset")}, "network"},
		"credentials":               {errors.New("failed to get credentials"), "other"},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, errorType(tc.err))
		})
	}
}

func TestTelemetryRecordUpload(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tel, tt := newTestTelemetry(t)

		start := tel.StartUpload(t.Context())
		require.False(t, start.IsZero())
		tel.RecordUpload(t.Context(), 11, start, nil)

		metadatatest.AssertEqualAwss3UploadedSize(t, tt, []metricdata.DataPoint[int64]{
			{Attributes: attribute.NewSet(), Value: 11},
		}, metricdatatest.IgnoreTimestamp())
		assertLatency(t, tt, successSet())
	})

	t.Run("a failure records the latency but adds no bytes", func(t *testing.T) {
		tel, tt := newTestTelemetry(t)

		tel.RecordUpload(t.Context(), 7, tel.StartUpload(t.Context()), errors.New("failed"))
		tel.RecordUpload(t.Context(), 11, tel.StartUpload(t.Context()), nil)

		// Only the successful upload adds bytes.
		metadatatest.AssertEqualAwss3UploadedSize(t, tt, []metricdata.DataPoint[int64]{
			{Attributes: attribute.NewSet(), Value: 11},
		}, metricdatatest.IgnoreTimestamp())
		assertLatency(t, tt, failureSet("other"), successSet())
	})

	t.Run("without a start nothing is recorded", func(t *testing.T) {
		tel, tt := newTestTelemetry(t)

		// Neither a success nor a failure is recorded, which a series of the
		// outcome or the bytes of the first calls would show.
		tel.RecordUpload(t.Context(), 100, time.Time{}, nil)
		tel.RecordUpload(t.Context(), 100, time.Time{}, errors.New("failed"))
		tel.RecordUpload(t.Context(), 11, tel.StartUpload(t.Context()), nil)

		metadatatest.AssertEqualAwss3UploadedSize(t, tt, []metricdata.DataPoint[int64]{
			{Attributes: attribute.NewSet(), Value: 11},
		}, metricdatatest.IgnoreTimestamp())
		assertLatency(t, tt, successSet())
	})
}

// TestTelemetryNotRecording covers the two ways of not recording: no telemetry at
// all, and a no-op meter provider, which is what the collector uses when the
// metrics are disabled. In both the request is not timed.
func TestTelemetryNotRecording(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	set.MeterProvider = noopmetric.NewMeterProvider()
	disabled, err := NewTelemetry(set)
	require.NoError(t, err)
	t.Cleanup(disabled.Shutdown)

	for name, tel := range map[string]*Telemetry{"nil": nil, "disabled": disabled} {
		t.Run(name, func(t *testing.T) {
			assert.True(t, tel.StartUpload(t.Context()).IsZero())
			assert.NotPanics(t, func() {
				tel.RecordUpload(t.Context(), 1, time.Now(), nil)
				tel.Shutdown()
			})
		})
	}
}

func TestWithTelemetryIgnoresOtherManagers(t *testing.T) {
	tel, _ := newTestTelemetry(t)
	assert.NotPanics(t, func() { WithTelemetry(tel)(nopManager{}) })
}

type nopManager struct{}

func (nopManager) Upload(context.Context, []byte, *UploadOptions) error { return nil }

// TestS3ManagerUploadTelemetry checks the telemetry of uploads done by the real
// manager against a stub S3, including the classification of the errors of the
// AWS SDK.
func TestS3ManagerUploadTelemetry(t *testing.T) {
	data := []byte(strings.Repeat("a log line to be compressed ", 100))

	respond := func(status int, code string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			if code != "" {
				_, _ = fmt.Fprintf(w, `<Error><Code>%s</Code><Message>m</Message></Error>`, code)
			}
		}
	}

	// On Windows a refused connection is the Winsock error WSAECONNREFUSED,
	// which syscall.ECONNREFUSED does not match, so it is classified as network.
	refusedErrType := "connection_refused"
	if runtime.GOOS == "windows" {
		refusedErrType = "network"
	}

	for name, tc := range map[string]struct {
		compression configcompression.Type
		handler     http.HandlerFunc
		closed      bool
		wantErrType string // empty for a success
	}{
		"success":                 {handler: respond(http.StatusOK, "")},
		"success gzip":            {compression: configcompression.TypeGzip, handler: respond(http.StatusOK, "")},
		"access denied":           {handler: respond(http.StatusForbidden, "AccessDenied"), wantErrType: "AccessDenied"},
		"error without a code":    {handler: respond(http.StatusForbidden, ""), wantErrType: "Forbidden"},
		"connection refused":      {handler: respond(http.StatusOK, ""), closed: true, wantErrType: refusedErrType},
		"connection closed by S3": {handler: closeConnection, wantErrType: "network"},
		"slow S3 and a deadline":  {handler: func(http.ResponseWriter, *http.Request) { time.Sleep(time.Second) }, wantErrType: "timeout"},
	} {
		t.Run(name, func(t *testing.T) {
			s := httptest.NewServer(tc.handler)
			t.Cleanup(s.Close)
			endpoint := s.URL
			if tc.closed {
				s.Close()
			}

			tel, tt := newTestTelemetry(t)
			sm := NewS3Manager(zap.NewNop(), "my-bucket",
				&PartitionKeyBuilder{PartitionFormat: "year=%Y", FilePrefix: "signal-data-", Metadata: "logs", FileFormat: "json", Compression: tc.compression},
				s3.New(s3.Options{BaseEndpoint: aws.String(endpoint), Region: "local", UsePathStyle: true, Retryer: aws.NopRetryer{}}),
				"STANDARD", WithTelemetry(tel))

			ctx := t.Context()
			if tc.wantErrType == "timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 50*time.Millisecond)
				t.Cleanup(cancel)
			}

			err := sm.Upload(ctx, data, nil)

			if tc.wantErrType != "" {
				require.Error(t, err)
				assertLatency(t, tt, failureSet(tc.wantErrType))
				return
			}

			require.NoError(t, err)
			// The bytes are the size of the body that is uploaded, once compressed.
			body, err := sm.(*s3manager).contentBuffer(data)
			require.NoError(t, err)
			if tc.compression != "" {
				require.Less(t, body.Len(), len(data))
			}
			metadatatest.AssertEqualAwss3UploadedSize(t, tt, []metricdata.DataPoint[int64]{
				{Attributes: attribute.NewSet(), Value: int64(body.Len())},
			}, metricdatatest.IgnoreTimestamp())
			assertLatency(t, tt, successSet())
		})
	}
}

func closeConnection(w http.ResponseWriter, _ *http.Request) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err == nil {
		_ = conn.Close()
	}
}
