// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"

import (
	"context"
	"fmt"
	"sync"

	stefgrpc "github.com/splunk/stef/go/grpc"
	"github.com/splunk/stef/go/grpc/stef_proto"
	"github.com/splunk/stef/go/otel/oteltef"
	stefpdatametrics "github.com/splunk/stef/go/pdata/metrics"
	stefpkg "github.com/splunk/stef/go/pkg"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter/internal"
)

// stefExporter implements sending metrics over STEF/gRPC stream.
//
// The exporter uses a single stream and accepts concurrent exportMetrics calls,
// sequencing the metric data as needed over a single stream.
//
// The exporter will block exportMetrics call until an acknowledgement is
// received from destination.
//
// The exporter relies on a preceding Retry helper to retry sending data that is
// not acknowledged or otherwise fails to be sent. The exporter will not retry
// sending the data itself.
type stefExporter struct {
	set         component.TelemetrySettings
	cfg         Config
	compression stefpkg.Compression

	// connMutex is taken when connecting, disconnecting or checking connection status.
	connMutex   sync.Mutex
	isConnected bool
	connID      uint64
	grpcConn    *grpc.ClientConn
	client      *stefgrpc.Client
	connCancel  context.CancelFunc

	// The STEF writer we write metrics to and which in turns sends them over gRPC.
	stefWriter      *oteltef.MetricsWriter
	stefWriterMutex sync.Mutex // protects stefWriter

	// lastAckID is the maximum ack ID received so far.
	lastAckID uint64
	// Cond to protect and signal lastAckID.
	ackCond *internal.CancellableCond
}

type loggerWrapper struct {
	logger *zap.Logger
}

func (w *loggerWrapper) Debugf(_ context.Context, format string, v ...any) {
	w.logger.Debug(fmt.Sprintf(format, v...))
}

func (w *loggerWrapper) Errorf(_ context.Context, format string, v ...any) {
	w.logger.Error(fmt.Sprintf(format, v...))
}

func newStefExporter(set component.TelemetrySettings, cfg *Config) *stefExporter {
	exp := &stefExporter{
		set:     set,
		cfg:     *cfg,
		ackCond: internal.NewCancellableCond(),
	}

	exp.compression = stefpkg.CompressionNone
	if exp.cfg.Compression == "zstd" {
		exp.compression = stefpkg.CompressionZstd
	}
	// Disable built-in grpc compression. STEF has its own zstd compression support.
	exp.cfg.Compression = ""
	return exp
}

func (s *stefExporter) Start(ctx context.Context, host component.Host) error {
	// Prepare gRPC connection.
	var err error
	s.grpcConn, err = s.cfg.ToClientConn(ctx, host, s.set)
	if err != nil {
		return err
	}

	// No need to block Start(), we will begin connection attempt in a goroutine.
	go func() {
		if err := s.ensureConnected(ctx); err != nil {
			s.set.Logger.Error("Error connecting to destination", zap.Error(err))
			// This is not a fatal error. exportMetrics() will try to connect again as needed.
		}
	}()
	return nil
}

func (s *stefExporter) Shutdown(ctx context.Context) error {
	s.disconnect(ctx)
	if s.grpcConn != nil {
		if err := s.grpcConn.Close(); err != nil {
			s.set.Logger.Error("Failed to close grpc connection", zap.Error(err))
		}
		s.grpcConn = nil
	}
	return nil
}

func (s *stefExporter) ensureConnected(ctx context.Context) error {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	if s.isConnected {
		return nil
	}

	s.set.Logger.Debug("Connecting to destination", zap.String("endpoint", s.cfg.Endpoint))

	s.ackCond.Cond.L.Lock()
	// Reset lastAckID. New STEF stream ack IDs will start from 1.
	s.lastAckID = 0
	// Increment connection ID, to make sure we don't confuse the new and
	// previous (stale) connections.
	s.connID++
	connID := s.connID
	s.ackCond.Cond.L.Unlock()

	// Prepare to open a STEF/gRPC stream to the server.
	grpcClient := stef_proto.NewSTEFDestinationClient(s.grpcConn)

	// Let server know about our schema.
	schema, err := oteltef.MetricsWireSchema()
	if err != nil {
		return err
	}

	settings := stefgrpc.ClientSettings{
		Logger:       &loggerWrapper{s.set.Logger},
		GrpcClient:   grpcClient,
		ClientSchema: &schema,
		Callbacks: stefgrpc.ClientCallbacks{
			OnAck: func(ackId uint64) error { return s.onGrpcAck(connID, ackId) },
		},
	}
	s.client = stefgrpc.NewClient(settings)

	s.connCancel = nil
	connCtx, connCancel := context.WithCancel(context.Background())

	connectionAttemptDone := make(chan struct{})
	defer close(connectionAttemptDone)

	// Start a goroutine that waits for success, failure or cancellation of
	// the connection attempt.
	go func() {
		// Wait for either connection attempt to be done or for the caller
		// of ensureConnected() to give up.
		select {
		case <-ctx.Done():
			// The caller of ensureConnected() cancelled while we are waiting
			// for connection to be established. We have to cancel the
			// connection attempt (and the whole connection if it raced us and
			// managed to connect - we will reconnect later again in that case).
			s.set.Logger.Debug("Canceling connection context because ensureConnected() caller cancelled.")
			connCancel()
		case <-connectionAttemptDone:
			// Connection attempt finished (successfully or no). No need to wait for the
			// previous case, calling connCancel() is not needed anymore now. It will be
			// called later, when disconnecting.
			// From this moment we are essentially detaching from the Context
			// that passed to ensureConnected() since we wanted to honor it only
			// for the duration of the connection attempt, but not for the duration
			// of the entire existence of the connection.
		}
	}()

	grpcWriter, opts, err := s.client.Connect(connCtx)
	if err != nil {
		connCancel()
		return fmt.Errorf("failed to connect to destination: %w", err)
	}

	opts.Compression = s.compression

	// Create STEF record writer over gRPC.
	s.stefWriter, err = oteltef.NewMetricsWriter(grpcWriter, opts)
	if err != nil {
		connCancel()
		return err
	}

	// From this point on we consider the connection successfully established.
	s.isConnected = true

	// We need to call the cancel func when this connection is over so that we don't
	// leak the Context we just created. This will be done in disconnect().
	s.connCancel = connCancel

	s.set.Logger.Debug("Connected to destination", zap.String("endpoint", s.cfg.Endpoint))

	return nil
}

func (s *stefExporter) disconnect(ctx context.Context) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	if !s.isConnected {
		return
	}

	if s.connCancel != nil {
		s.set.Logger.Debug("Calling cancel on connection context to avoid leaks")
		s.connCancel()
		s.connCancel = nil
	}

	if err := s.client.Disconnect(ctx); err != nil {
		s.set.Logger.Error("Failed to disconnect", zap.Error(err))
	}

	s.set.Logger.Debug("Disconnected.")
	s.isConnected = false
}

func (s *stefExporter) exportMetrics(ctx context.Context, md pmetric.Metrics) error {
	if err := s.ensureConnected(ctx); err != nil {
		return err
	}

	// stefWriter is not safe for concurrent writing, protect it.
	s.stefWriterMutex.Lock()
	defer s.stefWriterMutex.Unlock()

	converter := stefpdatametrics.OtlpToSTEFUnsorted{}
	err := converter.WriteMetrics(md, s.stefWriter)
	if err != nil {
		s.set.Logger.Debug("WriteMetrics failed", zap.Error(err))

		// Error to write to STEF stream typically indicates either:
		// 1) A problem with the connection. We need to reconnect.
		// 2) Encoding failure, possibly due to encoder bug. In this case
		//    we need to reconnect too, to make sure encoders start from
		//    initial state, which is our best chance to succeed next time.
		//
		// We need to reconnect. Disconnect here and the next exportMetrics()
		// call will connect again.
		s.disconnect(ctx)

		// TODO: check if err is because STEF encoding failed. If so we must not
		// try to re-encode the same data. Return consumererror.NewPermanent(err)
		// to the caller. This requires changes in STEF Go library.

		// Return an error to retry sending these metrics again next time.
		return err
	}

	// According to STEF gRPC spec the destination ack IDs match written record number.
	// When the data we have just written is received by destination it will send us
	// back and ack ID that numerically matches the last written record number.
	expectedAckID := s.stefWriter.RecordCount()

	// stefWriter normally buffers written records in memory. Flush() ensures buffered
	// data is sent to network. This is necessary so that the server receives it and
	// sends an acknowledgement back.
	if err = s.stefWriter.Flush(); err != nil {
		s.set.Logger.Debug("Flush failed", zap.Error(err))

		// Failure to write the gRPC stream normally means something is
		// wrong with the connection. We need to reconnect. Disconnect here
		// and the next exportMetrics() call will connect again.
		s.disconnect(ctx)

		// Return an error to retry sending these metrics again next time.
		return err
	}

	// Wait for acknowledgement.
	err = s.ackCond.Wait(ctx, func() bool { return s.lastAckID >= expectedAckID })
	if err != nil {
		return fmt.Errorf("error waiting for ack ID %d: %w", expectedAckID, err)
	}

	return nil
}

func (s *stefExporter) onGrpcAck(connID uint64, ackID uint64) error {
	s.ackCond.Cond.L.Lock()
	defer s.ackCond.Cond.L.Unlock()

	if s.connID != connID {
		// This is an ack from a previous (stale) connection. This can happen
		// due to a race if the ack from the old stream arrives after we decided
		// to reconnect but the old stream is still functioning. We just need
		// to ignore this ack, it is no longer relevant.
		return nil
	}

	// The IDs are expected to always monotonically increase. Check it anyway in case
	// the server misbehaves and violates the expectation.
	if s.lastAckID < ackID {
		s.lastAckID = ackID
		s.ackCond.Cond.Broadcast()
	}
	return nil
}
