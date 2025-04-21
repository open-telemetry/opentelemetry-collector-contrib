// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	stefgrpc "github.com/splunk/stef/go/grpc"
	"github.com/splunk/stef/go/grpc/stef_proto"
	"github.com/splunk/stef/go/otel/oteltef"
	"github.com/splunk/stef/go/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func newGrpcServer(listener net.Listener) (*grpc.Server, int) {
	serverPort := listener.Addr().(*net.TCPAddr).Port
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	return grpcServer, serverPort
}

type mockMetricDestServer struct {
	stef_proto.UnimplementedSTEFDestinationServer
	logger          *zap.Logger
	grpcServer      *grpc.Server
	recordsReceived atomic.Int64
	acksSent        atomic.Int64
	endpoint        string
	failAckCount    atomic.Int64
}

func newMockMetricDestServer(t *testing.T, logger *zap.Logger) *mockMetricDestServer {
	m := &mockMetricDestServer{logger: logger}
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	m.endpoint = tcpAddr
	return m
}

func (m *mockMetricDestServer) start() {
	listener, err := net.Listen("tcp", m.endpoint)
	if err != nil {
		m.logger.Fatal("Failed to find an available address to run the gRPC server", zap.Error(err))
	}

	grpcServer, serverPort := newGrpcServer(listener)
	m.logger.Info("Listening for connections", zap.Int("port", serverPort))

	m.grpcServer = grpcServer

	schema, err := oteltef.MetricsWireSchema()
	if err != nil {
		m.logger.Fatal("Failed to load schema", zap.Error(err))
	}

	settings := stefgrpc.ServerSettings{
		Logger:       nil,
		ServerSchema: &schema,
		MaxDictBytes: 0,
		OnStream:     m.onStream,
	}
	mockServer := stefgrpc.NewStreamServer(settings)
	stef_proto.RegisterSTEFDestinationServer(grpcServer, mockServer)
	go func() {
		err := grpcServer.Serve(listener)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			m.logger.Fatal("Failed to start STEF server", zap.Error(err))
		}
	}()
}

func (m *mockMetricDestServer) stop() {
	m.grpcServer.Stop()
}

func (m *mockMetricDestServer) onStream(grpcReader stefgrpc.GrpcReader, ackFunc func(sequenceId uint64) error) error {
	m.logger.Info("Incoming TEF/gRPC connection.")

	reader, err := oteltef.NewMetricsReader(grpcReader)
	if err != nil {
		m.logger.Error("Error creating metrics reader from connection", zap.Error(err))
		return err
	}

	for {
		err = reader.Read(pkg.ReadOptions{})
		if err != nil {
			m.logger.Error("Error reading from connection", zap.Error(err))
			return err
		}
		m.recordsReceived.Add(1)

		if m.failAckCount.Add(-1) >= 0 {
			// This connection must fail to ack.
			continue
		}

		if err = ackFunc(reader.RecordCount()); err != nil {
			return err
		}
		m.acksSent.Add(1)
	}
}

func runTest(
	t *testing.T,
	cfg *Config,
	f func(cfg *Config, mockSrv *mockMetricDestServer, exp exporter.Metrics),
) {
	logCfg := zap.NewDevelopmentConfig()
	logCfg.DisableStacktrace = true
	logger, _ := logCfg.Build()

	mockSrv := newMockMetricDestServer(t, logger)

	mockSrv.start()
	defer mockSrv.stop()

	// Start an exporter and point to the server.
	factory := NewFactory()
	if cfg == nil {
		cfg = factory.CreateDefaultConfig().(*Config)
	}
	cfg.Endpoint = mockSrv.endpoint
	// Use insecure mode for tests so that we don't bother with certificates.
	cfg.TLSSetting.Insecure = true

	// Make retries quick. We will be testing failure modes and don't want test to take too long.
	cfg.RetryConfig.InitialInterval = 10 * time.Millisecond

	set := exportertest.NewNopSettings(metadata.Type)
	set.Logger = logger

	exp, err := factory.CreateMetrics(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()
	require.NoError(t, exp.Start(context.Background(), host))

	f(cfg, mockSrv, exp)
}

func TestExport(t *testing.T) {
	compressions := []string{"", "zstd"}
	for _, compression := range compressions {
		t.Run(
			compression, func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				cfg.Compression = configcompression.Type(compression)

				runTest(
					t,
					cfg,
					func(
						cfg *Config, mockSrv *mockMetricDestServer, exp exporter.Metrics,
					) {
						// Send some metrics. Make sure the count of batches
						// exceeds the number of consumers so that we can hit
						// the case where exporter begins to forcedly flush
						// encoded data.
						pointCount := int64(0)
						for i := 0; i < 2*cfg.QueueConfig.NumConsumers; i++ {
							md := testdata.GenerateMetrics(1)
							pointCount += int64(md.DataPointCount())
							err := exp.ConsumeMetrics(context.Background(), md)
							require.NoError(t, err)
						}

						// Wait for data to be received.
						assert.Eventually(
							t,
							func() bool { return mockSrv.recordsReceived.Load() == pointCount },
							5*time.Second, 5*time.Millisecond,
						)
					},
				)
			},
		)
	}
}

func TestReconnect(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Shorten max ack waiting time so that the attempt to send on a failed
	// connection times out quickly and attempt to send again is tried
	// until the broken connection is detected and reconnection happens.
	cfg.Timeout = 300 * time.Millisecond

	runTest(
		t,
		cfg,
		func(_ *Config, mockSrv *mockMetricDestServer, exp exporter.Metrics) {
			mockSrv.logger.Debug("======== 1")

			md := testdata.GenerateMetrics(1)
			pointCount := int64(md.DataPointCount())
			err := exp.ConsumeMetrics(context.Background(), md)
			require.NoError(t, err)

			// Wait for data to be received.
			assert.Eventually(
				t, func() bool { return mockSrv.recordsReceived.Load() == pointCount },
				5*time.Second, 5*time.Millisecond,
			)

			mockSrv.logger.Debug("First set of data received.")

			// Disconnect from server side to verify that the exporter will reconnect
			mockSrv.logger.Debug("Restarting mock STEF server.")
			mockSrv.stop()
			mockSrv.start()

			mockSrv.logger.Debug("======== 2")

			// Send more data
			md = testdata.GenerateMetrics(1)
			pointCount += int64(md.DataPointCount())
			err = exp.ConsumeMetrics(context.Background(), md)
			require.NoError(t, err)

			// Wait for data to be received.
			assert.Eventually(
				t, func() bool { return mockSrv.recordsReceived.Load() == pointCount },
				5*time.Second, 5*time.Millisecond,
			)
		},
	)
}

func TestAckTimeout(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Shorten max ack waiting time so that tests run fast.
	// Increase this if the second eventually() below fails sporadically.
	cfg.Timeout = 300 * time.Millisecond

	runTest(
		t,
		cfg,
		func(_ *Config, mockSrv *mockMetricDestServer, exp exporter.Metrics) {
			md := testdata.GenerateMetrics(1)
			pointCount := int64(md.DataPointCount())

			// Fail to ack the first pointCount records. We want acks to succeed on the second try only.
			mockSrv.failAckCount.Store(pointCount)

			err := exp.ConsumeMetrics(context.Background(), md)
			require.NoError(t, err)

			// Wait for data to be received.
			assert.Eventually(
				t, func() bool { return mockSrv.recordsReceived.Load() >= pointCount },
				5*time.Second, 5*time.Millisecond,
			)

			mockSrv.logger.Debug("First set of data received. Should remain unacknowledged.")

			// Because ack was not made by the server, the exporter is going to timeout,
			// reconnect and send the data again. The same data will be delivered again,
			// so recordsReceived counter will be twice the point count.
			assert.Eventually(
				t, func() bool { return mockSrv.recordsReceived.Load() == 2*pointCount },
				5*time.Second, 5*time.Millisecond,
			)

			mockSrv.logger.Debug("Second set of data received after reconnection. Should be acknowledged.")
			// Verify that acks were sent.
			assert.Equal(t, pointCount, mockSrv.acksSent.Load())
		},
	)
}

func TestStartServerAfterClient(t *testing.T) {
	logCfg := zap.NewDevelopmentConfig()
	logCfg.DisableStacktrace = true
	logger, _ := logCfg.Build()

	mockSrv := newMockMetricDestServer(t, logger)

	// Start an exporter and point to the server.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: mockSrv.endpoint,
		// Use insecure mode for tests so that we don't bother with certificates.
		TLSSetting: configtls.ClientConfig{Insecure: true},
	}

	set := exportertest.NewNopSettings(metadata.Type)
	set.Logger = logger

	exp := newStefExporter(set.TelemetrySettings, cfg)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()
	require.NoError(t, exp.Start(context.Background(), host))

	// Trying sending with server down.
	md := testdata.GenerateMetrics(1)
	pointCount := int64(md.DataPointCount())
	err := exp.exportMetrics(context.Background(), md)

	// Sending must fail.
	require.Error(t, err)

	// Now start the server.
	mockSrv.start()
	defer mockSrv.stop()

	// Try sending until it succeeds. The gRPC connection may not succeed immediately.
	assert.Eventually(
		t, func() bool {
			err = exp.exportMetrics(context.Background(), md)
			return err == nil
		}, 5*time.Second, 200*time.Millisecond,
	)

	// Ensure data is received.
	assert.Equal(t, pointCount, mockSrv.recordsReceived.Load())
}

func TestCancelBlockedExport(t *testing.T) {
	logCfg := zap.NewDevelopmentConfig()
	logCfg.DisableStacktrace = true
	logger, _ := logCfg.Build()

	// Listen but don't accept connections. This should block gRPC connection attempt.
	endpoint := testutil.GetAvailableLocalAddress(t)
	listener, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer listener.Close()

	// Start an exporter and point to the listener.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint:   endpoint,
		TLSSetting: configtls.ClientConfig{Insecure: true},
	}

	set := exportertest.NewNopSettings(exportertest.NopType)
	set.Logger = logger

	exp := newStefExporter(set.TelemetrySettings, cfg)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, exp.Start(ctx, host))

	// Cancel after Start() returns.
	// This should cancel the connection attempt that Start() initiated.
	go func() { cancel() }()

	md := testdata.GenerateMetrics(1)

	// Do some attempts send with cancellation to help trigger races if there is any.
	for i := 0; i < 10; i++ {
		// Trying sending with server down. The connection attempt will block
		// because listener does not accept connections. However exportMetrics()
		// will return almost immediately because connection attempt
		// context is cancelled.
		ctx, cancel := context.WithCancel(context.Background())
		go func() { cancel() }()
		err = exp.exportMetrics(ctx, md)

		// Sending must fail with Cancelled code.
		require.Error(t, err)
		stat, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Canceled, stat.Code())
	}
}

func TestCancelAfterExport(t *testing.T) {
	logCfg := zap.NewDevelopmentConfig()
	logCfg.DisableStacktrace = true
	logger, _ := logCfg.Build()

	mockSrv := newMockMetricDestServer(t, logger)

	// Start an exporter and point to the server.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: mockSrv.endpoint,
		// Use insecure mode for tests so that we don't bother with certificates.
		TLSSetting: configtls.ClientConfig{Insecure: true},
	}

	set := exportertest.NewNopSettings(exportertest.NopType)
	set.Logger = logger

	exp := newStefExporter(set.TelemetrySettings, cfg)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	// Start the server.
	mockSrv.start()
	defer mockSrv.stop()

	host := componenttest.NewNopHost()
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel the context to cause connection attempt in Start() to fail.
	cancel()
	require.NoError(t, exp.Start(ctx, host))

	var pointCount int64
	for i := 0; i < 10; i++ {
		md := testdata.GenerateMetrics(1)
		pointCount += int64(md.DataPointCount())
		ctx, cancel = context.WithCancel(context.Background())

		err := exp.exportMetrics(ctx, md)
		require.NoError(t, err)

		// Canceling context should not result in broken connection.
		// We had a bug in exporter implementation that was causing the next
		// exportMetrics() attempt to fail.
		cancel()
	}

	// Ensure all data is received.
	assert.Equal(t, pointCount, mockSrv.recordsReceived.Load())
}
