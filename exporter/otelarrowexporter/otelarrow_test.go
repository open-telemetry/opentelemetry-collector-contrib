// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter

import (
	"context"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowpbMock "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1/mock"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow/grpcmock"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testdata"
)

type mockReceiver struct {
	srv          *grpc.Server
	ln           net.Listener
	requestCount *atomic.Int32
	totalItems   *atomic.Int32
	mux          sync.Mutex
	metadata     metadata.MD
	exportError  error
}

func (r *mockReceiver) getMetadata() metadata.MD {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.metadata
}

func (r *mockReceiver) setExportError(err error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.exportError = err
}

type mockTracesReceiver struct {
	ptraceotlp.UnimplementedGRPCServer
	mockReceiver
	exportResponse      func() ptraceotlp.ExportResponse
	lastRequest         ptrace.Traces
	hasMetadata         bool
	spanCountByMetadata map[string]int
}

func (r *mockTracesReceiver) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	r.requestCount.Add(int32(1))
	td := req.Traces()
	r.totalItems.Add(int32(td.SpanCount()))
	r.mux.Lock()
	defer r.mux.Unlock()
	r.metadata, _ = metadata.FromIncomingContext(ctx)
	if r.hasMetadata {
		v1 := r.metadata.Get("key1")
		v2 := r.metadata.Get("key2")
		hashKey := fmt.Sprintf("%s|%s", v1, v2)
		r.spanCountByMetadata[hashKey] += (td.SpanCount())
	}
	r.lastRequest = td
	return r.exportResponse(), r.exportError
}

func (r *mockTracesReceiver) getLastRequest() ptrace.Traces {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.lastRequest
}

func (r *mockTracesReceiver) setExportResponse(fn func() ptraceotlp.ExportResponse) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.exportResponse = fn
}

func otelArrowTracesReceiverOnGRPCServer(ln net.Listener, useTLS bool) (*mockTracesReceiver, error) {
	sopts := []grpc.ServerOption{}

	if useTLS {
		_, currentFile, _, _ := runtime.Caller(0)
		basepath := filepath.Dir(currentFile)
		certpath := filepath.Join(basepath, filepath.Join("testdata", "test_cert.pem"))
		keypath := filepath.Join(basepath, filepath.Join("testdata", "test_key.pem"))

		creds, err := credentials.NewServerTLSFromFile(certpath, keypath)
		if err != nil {
			return nil, err
		}
		sopts = append(sopts, grpc.Creds(creds))
	}

	rcv := &mockTracesReceiver{
		mockReceiver: mockReceiver{
			srv:          grpc.NewServer(sopts...),
			ln:           ln,
			requestCount: &atomic.Int32{},
			totalItems:   &atomic.Int32{},
		},
		exportResponse: ptraceotlp.NewExportResponse,
	}

	ptraceotlp.RegisterGRPCServer(rcv.srv, rcv)

	return rcv, nil
}

func (r *mockTracesReceiver) start() {
	go func() {
		_ = r.srv.Serve(r.ln)
	}()
}

type mockLogsReceiver struct {
	plogotlp.UnimplementedGRPCServer
	mockReceiver
	exportResponse func() plogotlp.ExportResponse
	lastRequest    plog.Logs
}

func (r *mockLogsReceiver) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	r.requestCount.Add(int32(1))
	ld := req.Logs()
	r.totalItems.Add(int32(ld.LogRecordCount()))
	r.mux.Lock()
	defer r.mux.Unlock()
	r.lastRequest = ld
	r.metadata, _ = metadata.FromIncomingContext(ctx)
	return r.exportResponse(), r.exportError
}

func (r *mockLogsReceiver) getLastRequest() plog.Logs {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.lastRequest
}

func (r *mockLogsReceiver) setExportResponse(fn func() plogotlp.ExportResponse) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.exportResponse = fn
}

func otelArrowLogsReceiverOnGRPCServer(ln net.Listener) *mockLogsReceiver {
	rcv := &mockLogsReceiver{
		mockReceiver: mockReceiver{
			srv:          grpc.NewServer(),
			requestCount: &atomic.Int32{},
			totalItems:   &atomic.Int32{},
		},
		exportResponse: plogotlp.NewExportResponse,
	}

	// Now run it as a gRPC server
	plogotlp.RegisterGRPCServer(rcv.srv, rcv)
	go func() {
		_ = rcv.srv.Serve(ln)
	}()

	return rcv
}

type mockMetricsReceiver struct {
	pmetricotlp.UnimplementedGRPCServer
	mockReceiver
	exportResponse func() pmetricotlp.ExportResponse
	lastRequest    pmetric.Metrics
}

func (r *mockMetricsReceiver) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	md := req.Metrics()
	r.requestCount.Add(int32(1))
	r.totalItems.Add(int32(md.DataPointCount()))
	r.mux.Lock()
	defer r.mux.Unlock()
	r.lastRequest = md
	r.metadata, _ = metadata.FromIncomingContext(ctx)
	return r.exportResponse(), r.exportError
}

func (r *mockMetricsReceiver) getLastRequest() pmetric.Metrics {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.lastRequest
}

func (r *mockMetricsReceiver) setExportResponse(fn func() pmetricotlp.ExportResponse) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.exportResponse = fn
}

func otelArrowMetricsReceiverOnGRPCServer(ln net.Listener) *mockMetricsReceiver {
	rcv := &mockMetricsReceiver{
		mockReceiver: mockReceiver{
			srv:          grpc.NewServer(),
			requestCount: &atomic.Int32{},
			totalItems:   &atomic.Int32{},
		},
		exportResponse: pmetricotlp.NewExportResponse,
	}

	// Now run it as a gRPC server
	pmetricotlp.RegisterGRPCServer(rcv.srv, rcv)
	go func() {
		_ = rcv.srv.Serve(ln)
	}()

	return rcv
}

type hostWithExtensions struct {
	component.Host
	exts map[component.ID]component.Component
}

func newHostWithExtensions(exts map[component.ID]component.Component) component.Host {
	return &hostWithExtensions{
		Host: componenttest.NewNopHost(),
		exts: exts,
	}
}

func (h *hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return h.exts
}

var _ extensionauth.GRPCClient = (*testAuthExtension)(nil)

type testAuthExtension struct {
	extension.Extension

	prc credentials.PerRPCCredentials
}

func newTestAuthExtension(t *testing.T, mdf func(ctx context.Context) map[string]string) extension.Extension {
	ctrl := gomock.NewController(t)
	prc := grpcmock.NewMockPerRPCCredentials(ctrl)
	prc.EXPECT().RequireTransportSecurity().AnyTimes().Return(false)
	prc.EXPECT().GetRequestMetadata(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(ctx context.Context, _ ...string) (map[string]string, error) {
			return mdf(ctx), nil
		},
	)
	return &testAuthExtension{
		prc: prc,
	}
}

func (a *testAuthExtension) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return a.prc, nil
}

func TestSendTraces(t *testing.T) {
	// Start an OTel-Arrow receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv, _ := otelArrowTracesReceiverOnGRPCServer(ln, false)
	rcv.start()
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	authID := component.NewID(component.MustNewType("testauth"))
	expectedHeader := []string{"header-value"}

	cfg := factory.CreateDefaultConfig().(*Config)
	// Disable queuing to ensure that we execute the request when calling ConsumeTraces
	// otherwise we will not see any errors.
	cfg.QueueSettings.Enabled = false
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
		Headers: map[string]configopaque.String{
			"header": configopaque.String(expectedHeader[0]),
		},
		Auth: &configauth.Config{
			AuthenticatorID: authID,
		},
	}
	// This test fails w/ Arrow enabled because the function
	// passed to newTestAuthExtension() below requires it the
	// caller's context, and the newStream doesn't have it.
	cfg.Arrow.Disabled = true

	set := exportertest.NewNopSettings(factory.Type())
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	exp, err := factory.CreateTraces(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := newHostWithExtensions(
		map[component.ID]component.Component{
			authID: newTestAuthExtension(t, func(ctx context.Context) map[string]string {
				return map[string]string{
					"callerid": client.FromContext(ctx).Metadata.Get("in_callerid")[0],
				}
			}),
		},
	)
	assert.NoError(t, exp.Start(context.Background(), host))

	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	newCallerContext := func(value string) context.Context {
		return client.NewContext(context.Background(),
			client.Info{
				Metadata: client.NewMetadata(map[string][]string{
					"in_callerid": {value},
				}),
			},
		)
	}
	const caller1 = "caller1"
	const caller2 = "caller2"
	callCtx1 := newCallerContext(caller1)
	callCtx2 := newCallerContext(caller2)

	// Send empty trace.
	td := ptrace.NewTraces()
	assert.NoError(t, exp.ConsumeTraces(callCtx1, td))

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Ensure it was received empty.
	assert.EqualValues(t, 0, rcv.totalItems.Load())
	md := rcv.getMetadata()

	// Expect caller1 and the static header
	require.Equal(t, expectedHeader, md.Get("header"))
	require.Equal(t, []string{caller1}, md.Get("callerid"))

	// A trace with 2 spans.
	td = testdata.GenerateTraces(2)

	err = exp.ConsumeTraces(callCtx2, td)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond)

	// Verify received span.
	assert.EqualValues(t, 2, rcv.totalItems.Load())
	assert.EqualValues(t, 2, rcv.requestCount.Load())
	assert.Equal(t, td, rcv.getLastRequest())

	// Test the static metadata
	md = rcv.getMetadata()
	require.Equal(t, expectedHeader, md.Get("header"))
	require.Len(t, md.Get("User-Agent"), 1)
	require.Contains(t, md.Get("User-Agent")[0], "Collector/1.2.3test")

	// Test the caller's dynamic metadata
	require.Equal(t, []string{caller2}, md.Get("callerid"))

	// Return partial success
	rcv.setExportResponse(func() ptraceotlp.ExportResponse {
		response := ptraceotlp.NewExportResponse()
		partialSuccess := response.PartialSuccess()
		partialSuccess.SetErrorMessage("Some spans were not ingested")
		partialSuccess.SetRejectedSpans(1)

		return response
	})

	// A request with 2 Trace entries.
	td = testdata.GenerateTraces(2)

	// PartialSuccess is not an error.
	err = exp.ConsumeTraces(callCtx1, td)
	assert.NoError(t, err)
}

func TestSendTracesWhenEndpointHasHttpScheme(t *testing.T) {
	tests := []struct {
		name               string
		useTLS             bool
		scheme             string
		gRPCClientSettings configgrpc.ClientConfig
	}{
		{
			name:               "Use https scheme",
			useTLS:             true,
			scheme:             "https://",
			gRPCClientSettings: configgrpc.ClientConfig{},
		},
		{
			name:   "Use http scheme",
			useTLS: false,
			scheme: "http://",
			gRPCClientSettings: configgrpc.ClientConfig{
				TLS: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Start an OTel-Arrow receiver.
			ln, err := net.Listen("tcp", "localhost:")
			require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
			rcv, err := otelArrowTracesReceiverOnGRPCServer(ln, test.useTLS)
			rcv.start()
			require.NoError(t, err, "Failed to start mock OTLP receiver")
			// Also closes the connection.
			defer rcv.srv.GracefulStop()

			// Start an OTLP exporter and point to the receiver.
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.ClientConfig = test.gRPCClientSettings
			cfg.Endpoint = test.scheme + ln.Addr().String()
			cfg.Arrow.MaxStreamLifetime = 100 * time.Second
			if test.useTLS {
				cfg.TLS.InsecureSkipVerify = true
			}
			set := exportertest.NewNopSettings(factory.Type())
			exp, err := factory.CreateTraces(context.Background(), set, cfg)
			require.NoError(t, err)
			require.NotNil(t, exp)

			defer func() {
				assert.NoError(t, exp.Shutdown(context.Background()))
			}()

			host := componenttest.NewNopHost()
			assert.NoError(t, exp.Start(context.Background(), host))

			// Ensure that initially there is no data in the receiver.
			assert.EqualValues(t, 0, rcv.requestCount.Load())

			// Send empty trace.
			td := ptrace.NewTraces()
			assert.NoError(t, exp.ConsumeTraces(context.Background(), td))

			// Wait until it is received.
			assert.Eventually(t, func() bool {
				return rcv.requestCount.Load() > 0
			}, 10*time.Second, 5*time.Millisecond)

			// Ensure it was received empty.
			assert.EqualValues(t, 0, rcv.totalItems.Load())
		})
	}
}

func TestSendMetrics(t *testing.T) {
	// Start an OTel-Arrow receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv := otelArrowMetricsReceiverOnGRPCServer(ln)
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Disable queuing to ensure that we execute the request when calling ConsumeMetrics
	// otherwise we will not see any errors.
	cfg.QueueSettings.Enabled = false
	cfg.RetryConfig.Enabled = false
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
		Headers: map[string]configopaque.String{
			"header": "header-value",
		},
	}
	cfg.Arrow.MaxStreamLifetime = 100 * time.Second
	set := exportertest.NewNopSettings(factory.Type())
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	exp, err := factory.CreateMetrics(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	// Send empty metric.
	md := pmetric.NewMetrics()
	assert.NoError(t, exp.ConsumeMetrics(context.Background(), md))

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Ensure it was received empty.
	assert.EqualValues(t, 0, rcv.totalItems.Load())

	// Send two metrics.
	md = testdata.GenerateMetrics(2)

	err = exp.ConsumeMetrics(context.Background(), md)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond)

	expectedHeader := []string{"header-value"}

	// Verify received metrics.
	assert.EqualValues(t, uint32(2), rcv.requestCount.Load())
	assert.EqualValues(t, uint32(4), rcv.totalItems.Load())
	assert.Equal(t, md, rcv.getLastRequest())

	mdata := rcv.getMetadata()
	require.Equal(t, expectedHeader, mdata.Get("header"))
	require.Len(t, mdata.Get("User-Agent"), 1)
	require.Contains(t, mdata.Get("User-Agent")[0], "Collector/1.2.3test")

	st := status.New(codes.InvalidArgument, "Invalid argument")
	rcv.setExportError(st.Err())

	// Send two metrics..
	md = testdata.GenerateMetrics(2)

	err = exp.ConsumeMetrics(context.Background(), md)
	assert.Error(t, err)

	rcv.setExportError(nil)

	// Return partial success
	rcv.setExportResponse(func() pmetricotlp.ExportResponse {
		response := pmetricotlp.NewExportResponse()
		partialSuccess := response.PartialSuccess()
		partialSuccess.SetErrorMessage("Some data points were not ingested")
		partialSuccess.SetRejectedDataPoints(1)

		return response
	})

	// Send two metrics.
	md = testdata.GenerateMetrics(2)
	assert.NoError(t, exp.ConsumeMetrics(context.Background(), md))
}

func TestSendTraceDataServerDownAndUp(t *testing.T) {
	// Find the addr, but don't start the server.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTel-Arrow exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Disable queuing to ensure that we execute the request when calling ConsumeTraces
	// otherwise we will not see the error.
	cfg.QueueSettings.Enabled = false
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
		// Need to wait for every request blocking until either request timeouts or succeed.
		// Do not rely on external retry logic here, if that is intended set InitialInterval to 100ms.
		WaitForReady: true,
	}
	cfg.Arrow.MaxStreamLifetime = 100 * time.Second
	set := exportertest.NewNopSettings(factory.Type())
	exp, err := factory.CreateTraces(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// A trace with 2 spans.
	td := testdata.GenerateTraces(2)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	cancel()

	startServerAndMakeRequest(t, exp, td, ln)

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	cancel()

	// First call to startServerAndMakeRequest closed the connection. There is a race condition here that the
	// port may be reused, if this gets flaky rethink what to do.
	ln, err = net.Listen("tcp", ln.Addr().String())
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	startServerAndMakeRequest(t, exp, td, ln)

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	cancel()
}

func TestSendTraceDataServerStartWhileRequest(t *testing.T) {
	// Find the addr, but don't start the server.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTel-Arrow exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.Arrow.MaxStreamLifetime = 100 * time.Second
	set := exportertest.NewNopSettings(factory.Type())
	exp, err := factory.CreateTraces(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// A trace with 2 spans.
	td := testdata.GenerateTraces(2)
	done := make(chan bool, 1)
	defer close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		assert.NoError(t, exp.ConsumeTraces(ctx, td))
		done <- true
	}()

	time.Sleep(2 * time.Second)
	rcv, _ := otelArrowTracesReceiverOnGRPCServer(ln, false)
	rcv.start()
	defer rcv.srv.GracefulStop()
	// Wait until one of the conditions below triggers.
	select {
	case <-ctx.Done():
		t.Fail()
	case <-done:
		assert.NoError(t, ctx.Err())
	}
	cancel()
}

func TestSendTracesOnResourceExhaustion(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	rcv, _ := otelArrowTracesReceiverOnGRPCServer(ln, false)
	rcv.setExportError(status.Error(codes.ResourceExhausted, "resource exhausted"))
	rcv.start()
	defer rcv.srv.GracefulStop()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.RetryConfig.InitialInterval = 0
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.Arrow.MaxStreamLifetime = 100 * time.Second
	set := exportertest.NewNopSettings(factory.Type())
	exp, err := factory.CreateTraces(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()
	assert.NoError(t, exp.Start(context.Background(), host))

	assert.EqualValues(t, 0, rcv.requestCount.Load())

	td := ptrace.NewTraces()
	assert.NoError(t, exp.ConsumeTraces(context.Background(), td))

	assert.Never(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 1*time.Second, 5*time.Millisecond, "Should not retry if RetryInfo is not included into status details by the server.")

	rcv.requestCount.Swap(0)

	st := status.New(codes.ResourceExhausted, "resource exhausted")
	st, _ = st.WithDetails(&errdetails.RetryInfo{
		RetryDelay: durationpb.New(100 * time.Millisecond),
	})
	rcv.setExportError(st.Err())

	assert.NoError(t, exp.ConsumeTraces(context.Background(), td))

	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond, "Should retry if RetryInfo is included into status details by the server.")
}

func startServerAndMakeRequest(t *testing.T, exp exporter.Traces, td ptrace.Traces, ln net.Listener) {
	rcv, _ := otelArrowTracesReceiverOnGRPCServer(ln, false)
	rcv.start()
	defer rcv.srv.GracefulStop()
	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	// Clone the request and store as expected.
	expectedData := ptrace.NewTraces()
	td.CopyTo(expectedData)

	// Resend the request, this should succeed.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	assert.NoError(t, exp.ConsumeTraces(ctx, td))
	cancel()

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Verify received span.
	assert.EqualValues(t, 2, rcv.totalItems.Load())
	assert.Equal(t, expectedData, rcv.getLastRequest())
}

func TestSendLogData(t *testing.T) {
	// Start an OTel-Arrow receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv := otelArrowLogsReceiverOnGRPCServer(ln)
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTel-Arrow exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Disable queuing to ensure that we execute the request when calling ConsumeLogs
	// otherwise we will not see any errors.
	cfg.QueueSettings.Enabled = false
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.Arrow.MaxStreamLifetime = 100 * time.Second
	set := exportertest.NewNopSettings(factory.Type())
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	exp, err := factory.CreateLogs(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	// Send empty request.
	ld := plog.NewLogs()
	assert.NoError(t, exp.ConsumeLogs(context.Background(), ld))

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Ensure it was received empty.
	assert.EqualValues(t, 0, rcv.totalItems.Load())

	// A request with 2 log entries.
	ld = testdata.GenerateLogs(2)

	err = exp.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond)

	// Verify received logs.
	assert.EqualValues(t, 2, rcv.requestCount.Load())
	assert.EqualValues(t, 2, rcv.totalItems.Load())
	assert.Equal(t, ld, rcv.getLastRequest())

	md := rcv.getMetadata()
	require.Len(t, md.Get("User-Agent"), 1)
	require.Contains(t, md.Get("User-Agent")[0], "Collector/1.2.3test")

	st := status.New(codes.InvalidArgument, "Invalid argument")
	rcv.setExportError(st.Err())

	// A request with 2 log entries.
	ld = testdata.GenerateLogs(2)

	err = exp.ConsumeLogs(context.Background(), ld)
	assert.Error(t, err)

	rcv.setExportError(nil)

	// Return partial success
	rcv.setExportResponse(func() plogotlp.ExportResponse {
		response := plogotlp.NewExportResponse()
		partialSuccess := response.PartialSuccess()
		partialSuccess.SetErrorMessage("Some log records were not ingested")
		partialSuccess.SetRejectedLogRecords(1)

		return response
	})

	// A request with 2 log entries.
	ld = testdata.GenerateLogs(2)

	err = exp.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)
}

// TestSendArrowTracesNotSupported tests a successful OTel-Arrow export w/
// and without Arrow, w/ WaitForReady and without.
func TestSendArrowTracesNotSupported(t *testing.T) {
	for _, waitForReady := range []bool{true, false} {
		for _, available := range []bool{true, false} {
			t.Run(fmt.Sprintf("waitForReady=%v available=%v", waitForReady, available),
				func(t *testing.T) { testSendArrowTraces(t, waitForReady, available) })
		}
	}
}

func testSendArrowTraces(t *testing.T, clientWaitForReady, streamServiceAvailable bool) {
	// Start an OTel-Arrow receiver.
	ln, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTel-Arrow exporter and point to the receiver.
	factory := NewFactory()
	authID := component.NewID(component.MustNewType("testauth"))
	expectedHeader := []string{"arrow-ftw"}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
		WaitForReady: clientWaitForReady,
		Headers: map[string]configopaque.String{
			"header": configopaque.String(expectedHeader[0]),
		},
		Auth: &configauth.Config{
			AuthenticatorID: authID,
		},
	}
	// Arrow client is enabled, but the server doesn't support it.
	cfg.Arrow.NumStreams = 1
	cfg.Arrow.MaxStreamLifetime = 100 * time.Second
	cfg.QueueSettings.Enabled = false

	set := exportertest.NewNopSettings(factory.Type())
	set.Logger = zaptest.NewLogger(t)
	exp, err := factory.CreateTraces(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	type isUserCall struct{}

	host := newHostWithExtensions(
		map[component.ID]component.Component{
			authID: newTestAuthExtension(t, func(ctx context.Context) map[string]string {
				if ctx.Value(isUserCall{}) == nil {
					return nil
				}
				return map[string]string{
					"callerid": "arrow",
				}
			}),
		},
	)
	assert.NoError(t, exp.Start(context.Background(), host))

	rcv, _ := otelArrowTracesReceiverOnGRPCServer(ln, false)

	defer func() {
		// Shutdown before GracefulStop, because otherwise we
		// wait for a full stream lifetime instead of closing
		// after requests are served.
		assert.NoError(t, exp.Shutdown(context.Background()))
		rcv.srv.GracefulStop()
	}()

	if streamServiceAvailable {
		rcv.startStreamMockArrowTraces(t, okStatusFor)
	}

	// Delay the server start, slightly.
	go func() {
		time.Sleep(100 * time.Millisecond)
		rcv.start()
	}()

	// Send two trace items.
	td := testdata.GenerateTraces(2)

	// Set the context key indicating this is per-request state,
	// so the auth extension returns data.
	err = exp.ConsumeTraces(context.WithValue(context.Background(), isUserCall{}, true), td)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Verify two items, one request received.
	assert.Equal(t, int32(2), rcv.totalItems.Load())
	assert.Equal(t, int32(1), rcv.requestCount.Load())
	assert.Equal(t, td, rcv.getLastRequest())

	// Expect the correct metadata, with or without arrow.
	md := rcv.getMetadata()
	require.Equal(t, []string{"arrow"}, md.Get("callerid"))
	require.Equal(t, expectedHeader, md.Get("header"))
}

func okStatusFor(id int64) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:    id,
		StatusCode: arrowpb.StatusCode_OK,
	}
}

func failedStatusFor(id int64) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:       id,
		StatusCode:    arrowpb.StatusCode_INVALID_ARGUMENT,
		StatusMessage: "test failed",
	}
}

type anyStreamServer interface {
	Send(*arrowpb.BatchStatus) error
	Recv() (*arrowpb.BatchArrowRecords, error)
	grpc.ServerStream
}

func (r *mockTracesReceiver) startStreamMockArrowTraces(t *testing.T, statusFor func(int64) *arrowpb.BatchStatus) {
	ctrl := gomock.NewController(t)

	doer := func(server anyStreamServer) error {
		consumer := arrowRecord.NewConsumer()
		var hdrs []hpack.HeaderField
		hdrsDecoder := hpack.NewDecoder(4096, func(hdr hpack.HeaderField) {
			hdrs = append(hdrs, hdr)
		})
		for {
			records, err := server.Recv()
			if status, ok := status.FromError(err); ok && status.Code() == codes.Canceled {
				break
			}
			if err != nil {
				// No errors are allowed, except EOF.
				require.Equal(t, io.EOF, err)
				break
			}

			got, err := consumer.TracesFrom(records)
			require.NoError(t, err)

			// Reset and parse headers
			hdrs = nil
			_, err = hdrsDecoder.Write(records.Headers)
			require.NoError(t, err)
			md, ok := metadata.FromIncomingContext(server.Context())
			require.True(t, ok)

			for _, hf := range hdrs {
				md[hf.Name] = append(md[hf.Name], hf.Value)
			}

			// Place the metadata into the context, where
			// the test framework (independent of Arrow)
			// receives it.
			ctx := metadata.NewIncomingContext(context.Background(), md)

			for _, traces := range got {
				_, err := r.Export(ctx, ptraceotlp.NewExportRequestFromTraces(traces))
				require.NoError(t, err)
			}
			require.NoError(t, server.Send(statusFor(records.BatchId)))
		}
		return nil
	}

	type singleBinding struct {
		arrowpb.UnsafeArrowTracesServiceServer
		*arrowpbMock.MockArrowTracesServiceServer
	}
	svc := arrowpbMock.NewMockArrowTracesServiceServer(ctrl)

	arrowpb.RegisterArrowTracesServiceServer(r.srv, singleBinding{
		MockArrowTracesServiceServer: svc,
	})
	svc.EXPECT().ArrowTraces(gomock.Any()).Times(1).DoAndReturn(doer)
}

func TestSendArrowFailedTraces(t *testing.T) {
	// Start an OTel-Arrow receiver.
	ln, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTel-Arrow exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
		WaitForReady: true,
	}
	// Arrow client is enabled, but the server doesn't support it.
	cfg.Arrow = ArrowConfig{
		NumStreams:        1,
		MaxStreamLifetime: 100 * time.Second,
	}
	cfg.QueueSettings.Enabled = false

	set := exportertest.NewNopSettings(factory.Type())
	set.Logger = zaptest.NewLogger(t)
	exp, err := factory.CreateTraces(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	host := componenttest.NewNopHost()
	assert.NoError(t, exp.Start(context.Background(), host))

	rcv, _ := otelArrowTracesReceiverOnGRPCServer(ln, false)
	rcv.startStreamMockArrowTraces(t, failedStatusFor)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
		rcv.srv.GracefulStop()
	}()

	// Delay the server start, slightly.
	go func() {
		time.Sleep(100 * time.Millisecond)
		rcv.start()
	}()

	// Send two trace items.
	td := testdata.GenerateTraces(2)
	err = exp.ConsumeTraces(context.Background(), td)
	assert.ErrorContains(t, err, "test failed")

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Verify two items, one request received.
	assert.Equal(t, int32(2), rcv.totalItems.Load())
	assert.Equal(t, int32(1), rcv.requestCount.Load())
	assert.Equal(t, td, rcv.getLastRequest())
}

func TestUserDialOptions(t *testing.T) {
	// Start an OTel-Arrow receiver.
	ln, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTel-Arrow exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
		WaitForReady: true,
	}
	cfg.Arrow.Disabled = true
	cfg.QueueSettings.Enabled = false

	const testAgent = "test-user-agent (release=:+1:)"

	// This overrides the default provided in otelArrow.go
	cfg.UserDialOptions = []grpc.DialOption{
		grpc.WithUserAgent(testAgent),
	}

	set := exportertest.NewNopSettings(factory.Type())
	set.Logger = zaptest.NewLogger(t)
	exp, err := factory.CreateTraces(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()
	assert.NoError(t, exp.Start(context.Background(), host))

	td := testdata.GenerateTraces(2)

	rcv, _ := otelArrowTracesReceiverOnGRPCServer(ln, false)
	rcv.start()
	defer rcv.srv.GracefulStop()

	err = exp.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)

	require.Len(t, rcv.getMetadata().Get("User-Agent"), 1)
	require.Contains(t, rcv.getMetadata().Get("User-Agent")[0], testAgent)
}
