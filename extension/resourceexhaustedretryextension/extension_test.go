// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// newTestExtension creates an extension with nop telemetry for tests that do not
// assert metric values.
func newTestExtension(t *testing.T, cfg *Config) *resourceExhaustedRetryExtension {
	t.Helper()
	e, err := newExtension(cfg, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	return e
}

// newTelSettings returns TelemetrySettings backed by a real SDK meter provider
// and a reader that tests can collect from to assert metric values.
func newTelSettings(t *testing.T) (component.TelemetrySettings, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	settings := componenttest.NewNopTelemetrySettings()
	settings.MeterProvider = mp
	t.Cleanup(func() { _ = mp.Shutdown(t.Context()) })
	return settings, reader
}

// newTelExtension creates an extension with a real meter provider wired up.
func newTelExtension(t *testing.T, cfg *Config) (*resourceExhaustedRetryExtension, *sdkmetric.ManualReader) {
	t.Helper()
	settings, reader := newTelSettings(t)
	e, err := newExtension(cfg, settings)
	require.NoError(t, err)
	return e, reader
}

const (
	metricRetriesSet    = "otelcol_extension_resource_exhausted_retry_retries_set"
	metricRetriesNotSet = "otelcol_extension_resource_exhausted_retry_retries_not_set"
	metricRetryDelay    = "otelcol_extension_resource_exhausted_retry_retry_delay"
)

func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &rm))
	return rm
}

// counterTotal returns the sum of all data-point values for the retries_set counter.
func counterTotal(rm metricdata.ResourceMetrics) int64 {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != metricRetriesSet {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				return 0
			}
			var total int64
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
			return total
		}
	}
	return 0
}

// counterByReason returns the value for the retries_not_set counter whose
// "reason" attribute matches the given value.
func counterByReason(rm metricdata.ResourceMetrics, reason string) int64 {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != metricRetriesNotSet {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				return 0
			}
			for _, dp := range sum.DataPoints {
				for _, attr := range dp.Attributes.ToSlice() {
					if string(attr.Key) == "reason" && attr.Value.AsString() == reason {
						return dp.Value
					}
				}
			}
		}
	}
	return 0
}

// histogramDataPoint finds the first data point for the retry_delay histogram.
func histogramDataPoint(rm metricdata.ResourceMetrics) (metricdata.HistogramDataPoint[int64], bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != metricRetryDelay {
				continue
			}
			hist, ok := m.Data.(metricdata.Histogram[int64])
			if !ok || len(hist.DataPoints) == 0 {
				return metricdata.HistogramDataPoint[int64]{}, false
			}
			return hist.DataPoints[0], true
		}
	}
	return metricdata.HistogramDataPoint[int64]{}, false
}

// serveStatus is a test helper that creates a middleware handler from the extension,
// serves a single request that responds with statusCode, and returns the recorded response.
func serveStatus(t *testing.T, e *resourceExhaustedRetryExtension, statusCode int) *httptest.ResponseRecorder {
	t.Helper()
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
	})
	wrapFn, err := e.GetHTTPHandler(t.Context())
	require.NoError(t, err)
	var handler http.Handler
	if wrapFn != nil {
		handler, err = wrapFn(t.Context(), base)
		require.NoError(t, err)
	} else {
		handler = base
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	handler.ServeHTTP(rec, req)
	return rec
}

// --- gRPC error wrapping ---

func TestWrapGRPCError_Disabled(t *testing.T) {
	e := newTestExtension(t, &Config{}) // RetryDelay == 0
	err := status.Error(codes.ResourceExhausted, "full")
	assert.Equal(t, err, e.wrapGRPCError(err))
}

func TestWrapGRPCError_NilError(t *testing.T) {
	e := newTestExtension(t, &Config{RetryDelay: time.Second})
	assert.NoError(t, e.wrapGRPCError(nil))
}

func TestWrapGRPCError_Permanent(t *testing.T) {
	e, reader := newTelExtension(t, &Config{RetryDelay: time.Second})
	inner := status.Error(codes.ResourceExhausted, "full")
	err := consumererror.NewPermanent(inner)

	assert.Equal(t, err, e.wrapGRPCError(err))

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(1), counterByReason(rm, "permanent"))
}

func TestWrapGRPCError_OtherCodes(t *testing.T) {
	e, reader := newTelExtension(t, &Config{RetryDelay: time.Second})

	for _, code := range []codes.Code{codes.Unavailable, codes.Internal, codes.InvalidArgument} {
		err := status.Error(code, "err")
		assert.Equal(t, err, e.wrapGRPCError(err), "code %v should not be modified", code)
	}

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(3), counterByReason(rm, "wrong_code"))
}

func TestWrapGRPCError_ExistingRetryInfo(t *testing.T) {
	e, reader := newTelExtension(t, &Config{RetryDelay: time.Second})
	st := status.New(codes.ResourceExhausted, "full")
	st, _ = st.WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(42 * time.Second)})
	err := st.Err()

	assert.Equal(t, err, e.wrapGRPCError(err))

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(1), counterByReason(rm, "retry_info_present"))
}

func TestWrapGRPCError_InjectsRetryInfo(t *testing.T) {
	delay := 2 * time.Second
	e, reader := newTelExtension(t, &Config{RetryDelay: delay})

	got := e.wrapGRPCError(status.Error(codes.ResourceExhausted, "full"))
	require.NotEqual(t, status.Error(codes.ResourceExhausted, "full"), got)

	st, ok := status.FromError(got)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	var ri *errdetails.RetryInfo
	for _, d := range st.Details() {
		if r, ok := d.(*errdetails.RetryInfo); ok {
			ri = r
		}
	}
	require.NotNil(t, ri, "RetryInfo not found in status details")
	assert.Equal(t, delay, ri.RetryDelay.AsDuration())

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(1), counterTotal(rm))
	dp, found := histogramDataPoint(rm)
	require.True(t, found, "retry_delay histogram not recorded")
	assert.Equal(t, uint64(1), dp.Count)
	assert.Equal(t, int64(2000), dp.Sum)
}

// --- gRPC server options ---

func TestGetGRPCServerOptions_Disabled(t *testing.T) {
	e := newTestExtension(t, &Config{})
	opts, err := e.GetGRPCServerOptions(t.Context())
	require.NoError(t, err)
	assert.Empty(t, opts)
}

func TestGetGRPCServerOptions_ReturnsInterceptors(t *testing.T) {
	e := newTestExtension(t, &Config{RetryDelay: time.Second})
	opts, err := e.GetGRPCServerOptions(t.Context())
	require.NoError(t, err)
	require.Len(t, opts, 2, "expected unary + stream interceptor options")
	require.NotPanics(t, func() { grpc.NewServer(opts...) })
}

// --- HTTP handler ---

func TestGetHTTPHandler_Disabled(t *testing.T) {
	e := newTestExtension(t, &Config{}) // RetryDelay == 0
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapFn, err := e.GetHTTPHandler(t.Context())
	require.NoError(t, err)
	// When disabled, the wrapper must pass through the base handler.
	require.NotNil(t, wrapFn)
	handler, err := wrapFn(t.Context(), base)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetHTTPHandler_Wraps(t *testing.T) {
	e, reader := newTelExtension(t, &Config{RetryDelay: time.Second})
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	wrapFn, err := e.GetHTTPHandler(t.Context())
	require.NoError(t, err)
	require.NotNil(t, wrapFn)
	handler, err := wrapFn(t.Context(), base)
	require.NoError(t, err)
	require.NotNil(t, handler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", http.NoBody))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(1), counterTotal(rm))
}

// --- HTTP middleware ---

func TestHTTPMiddleware_429_InjectsRetryAfter(t *testing.T) {
	e, reader := newTelExtension(t, &Config{RetryDelay: 2 * time.Second, Jitter: 0})

	rec := serveStatus(t, e, http.StatusTooManyRequests)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "2", rec.Header().Get("Retry-After"))

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(1), counterTotal(rm))
	dp, found := histogramDataPoint(rm)
	require.True(t, found, "retry_delay histogram not recorded")
	assert.Equal(t, uint64(1), dp.Count)
	assert.Equal(t, int64(2000), dp.Sum)
}

func TestHTTPMiddleware_OtherStatus_NoRetryAfter(t *testing.T) {
	e, reader := newTelExtension(t, &Config{RetryDelay: 2 * time.Second, Jitter: 0})

	for _, code := range []int{http.StatusOK, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		rec := serveStatus(t, e, code)
		assert.Equal(t, code, rec.Code)
		assert.Empty(t, rec.Header().Get("Retry-After"), "status %d should not have Retry-After", code)
	}

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(3), counterByReason(rm, "wrong_code"))
}

func TestHTTPMiddleware_SubSecondRoundsUpToOne(t *testing.T) {
	e, reader := newTelExtension(t, &Config{RetryDelay: 300 * time.Millisecond, Jitter: 0})

	rec := serveStatus(t, e, http.StatusTooManyRequests)
	assert.Equal(t, "1", rec.Header().Get("Retry-After"))

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(1), counterTotal(rm))
	dp, found := histogramDataPoint(rm)
	require.True(t, found)
	assert.Equal(t, uint64(1), dp.Count)
	assert.Equal(t, int64(300), dp.Sum)
}

func TestHTTPMiddleware_ExactSecondNoRoundup(t *testing.T) {
	e, reader := newTelExtension(t, &Config{RetryDelay: 2 * time.Second, Jitter: 0})

	rec := serveStatus(t, e, http.StatusTooManyRequests)
	assert.Equal(t, "2", rec.Header().Get("Retry-After"))

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(1), counterTotal(rm))
}

func TestHTTPMiddleware_429_Disabled(t *testing.T) {
	e := newTestExtension(t, &Config{}) // RetryDelay == 0
	rec := serveStatus(t, e, http.StatusTooManyRequests)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Empty(t, rec.Header().Get("Retry-After"))
}

func TestHTTPMiddleware_WriteHeader_DoubleCall(t *testing.T) {
	// countingWriter counts raw WriteHeader calls without its own once-guard,
	// so we can verify retryResponseWriter's own guard fires exactly once.
	calls := 0
	cw := &countingWriter{
		ResponseWriter: httptest.NewRecorder(),
		onWriteHeader:  func() { calls++ },
	}

	e, reader := newTelExtension(t, &Config{RetryDelay: 2 * time.Second})
	rw := &retryResponseWriter{ResponseWriter: cw, delay: 2 * time.Second, resource: e}
	rw.WriteHeader(http.StatusTooManyRequests) // first call
	rw.WriteHeader(http.StatusTooManyRequests) // second call — must be suppressed

	assert.Equal(t, 1, calls, "underlying WriteHeader must be called exactly once")
	assert.Equal(t, "2", cw.ResponseWriter.(*httptest.ResponseRecorder).Header().Get("Retry-After"))

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(1), counterTotal(rm),
		"suppressed second WriteHeader must not record a second metric")
}

// countingWriter is a test spy that tracks WriteHeader invocations.
type countingWriter struct {
	http.ResponseWriter
	onWriteHeader func()
}

func (cw *countingWriter) WriteHeader(code int) {
	cw.onWriteHeader()
	cw.ResponseWriter.WriteHeader(code)
}

func TestHTTPMiddleware_Jitter_429(t *testing.T) {
	base := 2 * time.Second
	jitter := 3 * time.Second
	e, reader := newTelExtension(t, &Config{RetryDelay: base, Jitter: jitter})

	for i := range 100 {
		rec := serveStatus(t, e, http.StatusTooManyRequests)
		val := rec.Header().Get("Retry-After")
		require.NotEmpty(t, val, "sample %d: Retry-After must be set", i)
		secs, err := strconv.Atoi(val)
		require.NoError(t, err, "sample %d: Retry-After must be an integer, got %q", i, val)
		assert.GreaterOrEqual(t, secs, int(base.Seconds()), "sample %d: Retry-After below RetryDelay", i)
		assert.LessOrEqual(t, secs, int((base + jitter).Seconds()), "sample %d: Retry-After above RetryDelay+Jitter", i)
	}

	rm := collectMetrics(t, reader)
	assert.Equal(t, int64(100), counterTotal(rm))
	dp, found := histogramDataPoint(rm)
	require.True(t, found, "retry_delay histogram not recorded")
	assert.Equal(t, uint64(100), dp.Count)
}
