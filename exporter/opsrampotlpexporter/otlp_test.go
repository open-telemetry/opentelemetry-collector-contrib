// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opsrampotlpexporter

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpc_creds "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

type mockReceiver struct {
	srv          *grpc.Server
	requestCount *atomic.Int32
	totalItems   *atomic.Int32
	mux          sync.Mutex
	metadata     metadata.MD
}

func (r *mockReceiver) GetMetadata() metadata.MD {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.metadata
}

type mockTracesReceiver struct {
	ptraceotlp.UnimplementedGRPCServer
	mockReceiver
	exportError error
	lastRequest ptrace.Traces
}

func (r *mockTracesReceiver) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	r.requestCount.Add(1)
	td := req.Traces()
	r.totalItems.Add(int32(td.SpanCount()))
	r.mux.Lock()
	defer r.mux.Unlock()
	r.lastRequest = td
	r.metadata, _ = metadata.FromIncomingContext(ctx)
	return ptraceotlp.NewExportResponse(), r.exportError
}

func (r *mockTracesReceiver) GetLastRequest() ptrace.Traces {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.lastRequest
}

func otlpTracesReceiverOnGRPCServer(ln net.Listener, useTLS bool) (*mockTracesReceiver, error) {
	sopts := []grpc.ServerOption{}

	if useTLS {
		_, currentFile, _, _ := runtime.Caller(0)
		basepath := filepath.Dir(currentFile)
		certpath := filepath.Join(basepath, filepath.Join("testdata", "test_cert.pem"))
		keypath := filepath.Join(basepath, filepath.Join("testdata", "test_key.pem"))

		creds, err := grpc_creds.NewServerTLSFromFile(certpath, keypath)
		if err != nil {
			return nil, err
		}
		sopts = append(sopts, grpc.Creds(creds))
	}

	rcv := &mockTracesReceiver{
		mockReceiver: mockReceiver{
			srv:          grpc.NewServer(sopts...),
			requestCount: &atomic.Int32{},
			totalItems:   &atomic.Int32{},
		},
	}

	// Now run it as a gRPC server
	ptraceotlp.RegisterGRPCServer(rcv.srv, rcv)
	go func() {
		_ = rcv.srv.Serve(ln)
	}()

	return rcv, nil
}

type mockLogsReceiver struct {
	plogotlp.UnimplementedGRPCServer
	mockReceiver
	lastRequest plog.Logs
}

func (r *mockLogsReceiver) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	r.requestCount.Add(1)
	ld := req.Logs()
	r.totalItems.Add(int32(ld.LogRecordCount()))
	r.mux.Lock()
	defer r.mux.Unlock()
	r.lastRequest = ld
	r.metadata, _ = metadata.FromIncomingContext(ctx)
	return plogotlp.NewExportResponse(), nil
}

func (r *mockLogsReceiver) GetLastRequest() plog.Logs {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.lastRequest
}

func otlpLogsReceiverOnGRPCServer(ln net.Listener) *mockLogsReceiver {
	rcv := &mockLogsReceiver{
		mockReceiver: mockReceiver{
			srv:          grpc.NewServer(),
			requestCount: &atomic.Int32{},
			totalItems:   &atomic.Int32{},
		},
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
	lastRequest pmetric.Metrics
}

func (r *mockMetricsReceiver) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	md := req.Metrics()
	r.requestCount.Add(1)
	r.totalItems.Add(int32(md.DataPointCount()))
	r.mux.Lock()
	defer r.mux.Unlock()
	r.lastRequest = md
	r.metadata, _ = metadata.FromIncomingContext(ctx)
	return pmetricotlp.NewExportResponse(), nil
}

func (r *mockMetricsReceiver) GetLastRequest() pmetric.Metrics {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.lastRequest
}

func otlpMetricsReceiverOnGRPCServer(ln net.Listener) *mockMetricsReceiver {
	rcv := &mockMetricsReceiver{
		mockReceiver: mockReceiver{
			srv:          grpc.NewServer(),
			requestCount: &atomic.Int32{},
			totalItems:   &atomic.Int32{},
		},
	}

	// Now run it as a gRPC server
	pmetricotlp.RegisterGRPCServer(rcv.srv, rcv)
	go func() {
		_ = rcv.srv.Serve(ln)
	}()

	return rcv
}

func TestSendTraces(t *testing.T) {
	// Start an OTLP-compatible receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.ClientConfig{
			Insecure: true,
		},
		Headers: map[string]configopaque.String{
			"header": "header-value",
		},
	}
	cfg.Security = SecuritySettings{
		OAuthServiceURL: "https://asura.opsramp.net/auth/oauth/token?agent=true",
		ClientID:        "mamRxRJB796HYtWYxqeDzeEXCKSswnsr",
		ClientSecret:    "Da2achZqvHF7tKDaSP3FCkHE2PKcY6twRxwZEnEYQHc5GADgHy5VZDBxdeKhNbrw",
	}
	set := exportertest.NewNopSettings()
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
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

	// A trace with 2 spans.
	td = testdata.GenerateTracesTwoSpansSameResource()

	err = exp.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond)

	expectedHeader := []string{"header-value"}

	// Verify received span.
	assert.EqualValues(t, 2, rcv.totalItems.Load())
	assert.EqualValues(t, 2, rcv.requestCount.Load())
	assert.EqualValues(t, td, rcv.GetLastRequest())

	md := rcv.GetMetadata()
	require.EqualValues(t, md.Get("header"), expectedHeader)
	require.Equal(t, len(md.Get("User-Agent")), 1)
	require.Contains(t, md.Get("User-Agent")[0], "Collector/1.2.3test")
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
				TLSSetting: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Start an OTLP-compatible receiver.
			ln, err := net.Listen("tcp", "localhost:")
			require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
			rcv, err := otlpTracesReceiverOnGRPCServer(ln, test.useTLS)
			require.NoError(t, err, "Failed to start mock OTLP receiver")
			// Also closes the connection.
			defer rcv.srv.GracefulStop()

			// Start an OTLP exporter and point to the receiver.
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Security = SecuritySettings{
				OAuthServiceURL: "https://asura.opsramp.net/auth/oauth/token?agent=true",
				ClientID:        "mamRxRJB796HYtWYxqeDzeEXCKSswnsr",
				ClientSecret:    "Da2achZqvHF7tKDaSP3FCkHE2PKcY6twRxwZEnEYQHc5GADgHy5VZDBxdeKhNbrw",
			}
			cfg.ClientConfig = test.gRPCClientSettings
			cfg.ClientConfig.Endpoint = test.scheme + ln.Addr().String()
			if test.useTLS {
				cfg.ClientConfig.TLSSetting.InsecureSkipVerify = true
			}
			set := exportertest.NewNopSettings()
			exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
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
	// Start an OTLP-compatible receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv := otlpMetricsReceiverOnGRPCServer(ln)
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.ClientConfig{
			Insecure: true,
		},
		Headers: map[string]configopaque.String{
			"header": "header-value",
		},
	}
	cfg.Security = SecuritySettings{
		OAuthServiceURL: "https://asura.opsramp.net/auth/oauth/token?agent=true",
		ClientID:        "mamRxRJB796HYtWYxqeDzeEXCKSswnsr",
		ClientSecret:    "Da2achZqvHF7tKDaSP3FCkHE2PKcY6twRxwZEnEYQHc5GADgHy5VZDBxdeKhNbrw",
	}
	set := exportertest.NewNopSettings()
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	exp, err := factory.CreateMetricsExporter(context.Background(), set, cfg)
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
	md = testdata.GenerateMetricsTwoMetrics()

	err = exp.ConsumeMetrics(context.Background(), md)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond)

	expectedHeader := []string{"header-value"}

	// Verify received metrics.
	assert.EqualValues(t, 2, rcv.requestCount.Load())
	assert.EqualValues(t, 4, rcv.totalItems.Load())
	assert.EqualValues(t, md, rcv.GetLastRequest())

	mdata := rcv.GetMetadata()
	require.EqualValues(t, mdata.Get("header"), expectedHeader)
	require.Equal(t, len(mdata.Get("User-Agent")), 1)
	require.Contains(t, mdata.Get("User-Agent")[0], "Collector/1.2.3test")
}

func TestSendTraceDataServerDownAndUp(t *testing.T) {
	// Find the addr, but don't start the server.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Disable queuing to ensure that we execute the request when calling ConsumeTraces
	// otherwise we will not see the error.
	cfg.QueueConfig.Enabled = false
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.ClientConfig{
			Insecure: true,
		},
		// Need to wait for every request blocking until either request timeouts or succeed.
		// Do not rely on external retry logic here, if that is intended set InitialInterval to 100ms.
		WaitForReady: true,
	}
	cfg.Security = SecuritySettings{
		OAuthServiceURL: "https://asura.opsramp.net/auth/oauth/token?agent=true",
		ClientID:        "mamRxRJB796HYtWYxqeDzeEXCKSswnsr",
		ClientSecret:    "Da2achZqvHF7tKDaSP3FCkHE2PKcY6twRxwZEnEYQHc5GADgHy5VZDBxdeKhNbrw",
	}

	set := exportertest.NewNopSettings()
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// A trace with 2 spans.
	td := testdata.GenerateTracesTwoSpansSameResource()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.EqualValues(t, context.DeadlineExceeded, ctx.Err())
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.EqualValues(t, context.DeadlineExceeded, ctx.Err())
	cancel()

	startServerAndMakeRequest(t, exp, td, ln)

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.EqualValues(t, context.DeadlineExceeded, ctx.Err())
	cancel()

	// First call to startServerAndMakeRequest closed the connection. There is a race condition here that the
	// port may be reused, if this gets flaky rethink what to do.
	ln, err = net.Listen("tcp", ln.Addr().String())
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	startServerAndMakeRequest(t, exp, td, ln)

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.EqualValues(t, context.DeadlineExceeded, ctx.Err())
	cancel()
}

func TestSendTraceDataServerStartWhileRequest(t *testing.T) {
	// Find the addr, but don't start the server.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.Security = SecuritySettings{
		OAuthServiceURL: "https://asura.opsramp.net/auth/oauth/token?agent=true",
		ClientID:        "mamRxRJB796HYtWYxqeDzeEXCKSswnsr",
		ClientSecret:    "Da2achZqvHF7tKDaSP3FCkHE2PKcY6twRxwZEnEYQHc5GADgHy5VZDBxdeKhNbrw",
	}
	set := exportertest.NewNopSettings()
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// A trace with 2 spans.
	td := testdata.GenerateTracesTwoSpansSameResource()
	done := make(chan bool, 1)
	defer close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		assert.NoError(t, exp.ConsumeTraces(ctx, td))
		done <- true
	}()

	time.Sleep(2 * time.Second)
	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
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
	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	rcv.exportError = status.Error(codes.ResourceExhausted, "resource exhausted")
	defer rcv.srv.GracefulStop()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.BackOffConfig.InitialInterval = 0
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.Security = SecuritySettings{
		OAuthServiceURL: "https://asura.opsramp.net/auth/oauth/token?agent=true",
		ClientID:        "mamRxRJB796HYtWYxqeDzeEXCKSswnsr",
		ClientSecret:    "Da2achZqvHF7tKDaSP3FCkHE2PKcY6twRxwZEnEYQHc5GADgHy5VZDBxdeKhNbrw",
	}
	set := exportertest.NewNopSettings()
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
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
	rcv.exportError = st.Err()

	assert.NoError(t, exp.ConsumeTraces(context.Background(), td))

	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond, "Should retry if RetryInfo is included into status details by the server.")
}

func startServerAndMakeRequest(t *testing.T, exp exporter.Traces, td ptrace.Traces, ln net.Listener) {
	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	defer rcv.srv.GracefulStop()
	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	// Clone the request and store as expected.
	var expectedData ptrace.Traces
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
	assert.EqualValues(t, expectedData, rcv.GetLastRequest())
}

func TestSendLogData(t *testing.T) {
	// Start an OTLP-compatible receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv := otlpLogsReceiverOnGRPCServer(ln)
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.Security = SecuritySettings{
		OAuthServiceURL: "https://asura.opsramp.net/auth/oauth/token?agent=true",
		ClientID:        "mamRxRJB796HYtWYxqeDzeEXCKSswnsr",
		ClientSecret:    "Da2achZqvHF7tKDaSP3FCkHE2PKcY6twRxwZEnEYQHc5GADgHy5VZDBxdeKhNbrw",
	}
	set := exportertest.NewNopSettings()
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	exp, err := factory.CreateLogsExporter(context.Background(), set, cfg)
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
	ld = testdata.GenerateLogsTwoLogRecordsSameResource()

	err = exp.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond)

	// Verify received logs.
	assert.EqualValues(t, 2, rcv.requestCount.Load())
	assert.EqualValues(t, 2, rcv.totalItems.Load())
	assert.EqualValues(t, ld, rcv.GetLastRequest())

	md := rcv.GetMetadata()
	require.Equal(t, len(md.Get("User-Agent")), 1)
	require.Contains(t, md.Get("User-Agent")[0], "Collector/1.2.3test")
}

func TestRegexp(t *testing.T) {
	strExp := "my 344 id is 123456"
	reStr := regexp.MustCompile(`\d+`)
	repStr := "${1}HIDDEN$2"
	output := reStr.ReplaceAllString(strExp, repStr)
	assert.Equal(t, output, "my HIDDEN id is HIDDEN")
}

func TestGetAuthToken(t *testing.T) {
	cfg := SecuritySettings{
		OAuthServiceURL: "https://asura.opsramp.net/auth/oauth/token?agent=true",
		ClientID:        "mamRxRJB796HYtWYxqeDzeEXCKSswnsr",
		ClientSecret:    "Da2achZqvHF7tKDaSP3FCkHE2PKcY6twRxwZEnEYQHc5GADgHy5VZDBxdeKhNbrw",
	}
	token, err := getAuthToken(cfg)
	assert.Nil(t, err)
	fmt.Println(token)
}

func TestSkipExpiredLogs(t *testing.T) {
	half := 30 * time.Minute
	tests := []struct {
		name       string
		expiration time.Duration
		expected   int
	}{
		{
			expiration: 1*time.Hour + half,
			expected:   2,
		},
		{
			expiration: 2*time.Hour + half,
			expected:   3,
		},
		{
			expiration: 3*time.Hour + half,
			expected:   4,
		},
		{
			expiration: 5*time.Hour + half,
			expected:   6,
		},
		{
			expiration: 9*time.Hour + half,
			expected:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld := generateTestEntries()
			e := opsrampOTLPExporter{config: &Config{ExpirationSkip: tt.expiration}}
			e.skipExpired(ld)
			assert.Equal(t, ld.LogRecordCount(), tt.expected)
		})
	}
}

func generateTestEntries() plog.Logs {
	ld := plog.NewLogs()
	rl0 := ld.ResourceLogs().AppendEmpty()
	sc := rl0.ScopeLogs().AppendEmpty()
	for i := 0; i < 10; i++ {
		el := sc.LogRecords().AppendEmpty()
		duration := time.Hour * time.Duration(i)
		el.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-duration)))
		el.Body().SetStr(fmt.Sprintf("This is entry # %q", i))
	}

	return ld
}
