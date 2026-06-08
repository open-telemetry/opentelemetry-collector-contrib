// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestWrapGRPCError_Disabled(t *testing.T) {
	e := newExtension(&Config{}) // RetryDelay == 0
	err := status.Error(codes.ResourceExhausted, "full")
	assert.Equal(t, err, e.wrapGRPCError(err))
}

func TestWrapGRPCError_NilError(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	assert.NoError(t, e.wrapGRPCError(nil))
}

func TestWrapGRPCError_Permanent(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	inner := status.Error(codes.ResourceExhausted, "full")
	err := consumererror.NewPermanent(inner)
	assert.Equal(t, err, e.wrapGRPCError(err))
}

func TestWrapGRPCError_OtherCodes(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	for _, code := range []codes.Code{codes.Unavailable, codes.Internal, codes.InvalidArgument} {
		err := status.Error(code, "err")
		assert.Equal(t, err, e.wrapGRPCError(err), "code %v should not be modified", code)
	}
}

func TestWrapGRPCError_ExistingRetryInfo(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	st := status.New(codes.ResourceExhausted, "full")
	st, _ = st.WithDetails(&errdetails.RetryInfo{
		RetryDelay: durationpb.New(42 * time.Second),
	})
	err := st.Err()
	got := e.wrapGRPCError(err)
	assert.Equal(t, err, got)
}

func TestWrapGRPCError_InjectsRetryInfo(t *testing.T) {
	delay := 2 * time.Second
	e := newExtension(&Config{RetryDelay: delay})
	err := status.Error(codes.ResourceExhausted, "full")

	got := e.wrapGRPCError(err)
	require.NotEqual(t, err, got)

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
}

func TestEffectiveDelay_NoJitter(t *testing.T) {
	e := newExtension(&Config{RetryDelay: 3 * time.Second})
	assert.Equal(t, 3*time.Second, e.effectiveDelay())
}

func TestEffectiveDelay_WithJitter(t *testing.T) {
	base := 2 * time.Second
	jitter := 3 * time.Second
	e := newExtension(&Config{RetryDelay: base, Jitter: jitter})
	for range 1000 {
		d := e.effectiveDelay()
		assert.GreaterOrEqual(t, d, base, "delay below RetryDelay")
		assert.LessOrEqual(t, d, base+jitter, "delay above RetryDelay+Jitter")
	}
}

func TestGetGRPCServerOptions_Disabled(t *testing.T) {
	e := newExtension(&Config{})
	opts, err := e.GetGRPCServerOptions()
	require.NoError(t, err)
	assert.Empty(t, opts)
}

func TestGetGRPCServerOptions_ReturnsInterceptors(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	opts, err := e.GetGRPCServerOptions()
	require.NoError(t, err)
	require.Len(t, opts, 2, "expected unary + stream interceptor options")
	require.NotPanics(t, func() { grpc.NewServer(opts...) })
}

// serveStatus is a test helper that creates a middleware handler from the extension,
// serves a single request that responds with statusCode, and returns the recorded response.
func serveStatus(t *testing.T, e *resourceExhaustedRetryExtension, statusCode int) *httptest.ResponseRecorder {
	t.Helper()
	handler, err := e.GetHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
	}))
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	handler.ServeHTTP(rec, req)
	return rec
}

func TestGetHTTPHandler_Disabled(t *testing.T) {
	e := newExtension(&Config{}) // RetryDelay == 0
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler, err := e.GetHTTPHandler(base)
	require.NoError(t, err)
	// When disabled, the base handler must be returned unchanged.
	// Compare function pointer addresses since func values cannot be compared directly.
	assert.Equal(t, fmt.Sprintf("%p", http.HandlerFunc(base)), fmt.Sprintf("%p", handler.(http.HandlerFunc)))
}

func TestGetHTTPHandler_Wraps(t *testing.T) {
	e := newExtension(&Config{RetryDelay: time.Second})
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	handler, err := e.GetHTTPHandler(base)
	require.NoError(t, err)
	require.NotNil(t, handler)
	// Serve a 429 and confirm Retry-After is injected — the handler must be a wrapping handler.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestHTTPMiddleware_429_InjectsRetryAfter(t *testing.T) {
	e := newExtension(&Config{RetryDelay: 2 * time.Second, Jitter: 0})
	rec := serveStatus(t, e, http.StatusTooManyRequests)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "2", rec.Header().Get("Retry-After"))
}

func TestHTTPMiddleware_OtherStatus_NoRetryAfter(t *testing.T) {
	e := newExtension(&Config{RetryDelay: 2 * time.Second, Jitter: 0})
	for _, code := range []int{http.StatusOK, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		rec := serveStatus(t, e, code)
		assert.Equal(t, code, rec.Code)
		assert.Empty(t, rec.Header().Get("Retry-After"), "status %d should not have Retry-After", code)
	}
}

func TestHTTPMiddleware_SubSecondRoundsUpToOne(t *testing.T) {
	e := newExtension(&Config{RetryDelay: 300 * time.Millisecond, Jitter: 0})
	rec := serveStatus(t, e, http.StatusTooManyRequests)
	assert.Equal(t, "1", rec.Header().Get("Retry-After"))
}

func TestHTTPMiddleware_ExactSecondNoRoundup(t *testing.T) {
	e := newExtension(&Config{RetryDelay: 2 * time.Second, Jitter: 0})
	rec := serveStatus(t, e, http.StatusTooManyRequests)
	assert.Equal(t, "2", rec.Header().Get("Retry-After"))
}

func TestHTTPMiddleware_429_Disabled(t *testing.T) {
	e := newExtension(&Config{}) // RetryDelay == 0
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

	rw := &retryResponseWriter{ResponseWriter: cw, delay: 2 * time.Second}
	rw.WriteHeader(http.StatusTooManyRequests) // first call
	rw.WriteHeader(http.StatusTooManyRequests) // second call — must be suppressed

	assert.Equal(t, 1, calls, "underlying WriteHeader must be called exactly once")
	assert.Equal(t, "2", cw.ResponseWriter.(*httptest.ResponseRecorder).Header().Get("Retry-After"))
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
	e := newExtension(&Config{RetryDelay: base, Jitter: jitter})

	for i := range 100 {
		rec := serveStatus(t, e, http.StatusTooManyRequests)
		val := rec.Header().Get("Retry-After")
		require.NotEmpty(t, val, "sample %d: Retry-After must be set", i)
		secs, err := strconv.Atoi(val)
		require.NoError(t, err, "sample %d: Retry-After must be an integer, got %q", i, val)
		assert.GreaterOrEqual(t, secs, int(base.Seconds()), "sample %d: Retry-After below RetryDelay", i)
		assert.LessOrEqual(t, secs, int((base + jitter).Seconds()), "sample %d: Retry-After above RetryDelay+Jitter", i)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "zero values valid",
			cfg:  Config{},
		},
		{
			name: "positive values valid",
			cfg:  Config{RetryDelay: 2 * time.Second, Jitter: 3 * time.Second},
		},
		{
			name:    "negative retry_delay",
			cfg:     Config{RetryDelay: -1 * time.Second},
			wantErr: "retry_delay must be non-negative",
		},
		{
			name:    "negative jitter",
			cfg:     Config{Jitter: -1 * time.Second},
			wantErr: "jitter must be non-negative",
		},
		{
			name:    "jitter exceeds 1h",
			cfg:     Config{Jitter: 2 * time.Hour},
			wantErr: "jitter must not exceed 1h",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
