// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter/internal"

import (
	"context"
	"fmt"
	"sync"

	stefgrpc "github.com/splunk/stef/go/grpc"
	"github.com/splunk/stef/go/grpc/stef_proto"
	"github.com/splunk/stef/go/otel/oteltef"
	"github.com/splunk/stef/go/pkg"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// StefConnCreator implements ConnCreator interface for STEF/gRPC connections.
type StefConnCreator struct {
	logger      *zap.Logger
	grpcConn    *grpc.ClientConn
	compression pkg.Compression
}

// StefConn implements Conn interface for STEF/gRPC connections.
type StefConn struct {
	client *stefgrpc.Client
	writer *oteltef.MetricsWriter

	// cancel func for the context that was used to create the gRPC stream.
	// Calling it will cancel stream operations.
	cancel context.CancelFunc

	// pendingAcks is a map of channels that are registered and awaiting
	// acknowledgments of the particular DataID.
	pendingAcks map[DataID]chan<- AsyncResult
	// mux protects the pendingAcks.
	mux sync.RWMutex

	flushReqCh chan struct{}
	flushResCh chan error
}

func NewStefConnCreator(logger *zap.Logger, grpcConn *grpc.ClientConn, compression pkg.Compression) *StefConnCreator {
	return &StefConnCreator{
		logger:      logger,
		grpcConn:    grpcConn,
		compression: compression,
	}
}

// Create a new connection. May be called concurrently.
// The attempt to create the connection should be cancelled if ctx is done.
func (s *StefConnCreator) Create(ctx context.Context) (Conn, error) {
	// Prepare to open a STEF/gRPC stream to the server.
	grpcClient := stef_proto.NewSTEFDestinationClient(s.grpcConn)

	// Let server know about our schema.
	schema, err := oteltef.MetricsWireSchema()
	if err != nil {
		return nil, err
	}

	conn := &StefConn{
		pendingAcks: map[DataID]chan<- AsyncResult{},
		flushReqCh:  make(chan struct{}, 1),
		flushResCh:  make(chan error, 1),
	}

	settings := stefgrpc.ClientSettings{
		Logger:       &loggerWrapper{s.logger},
		GrpcClient:   grpcClient,
		ClientSchema: stefgrpc.ClientSchema{WireSchema: &schema, RootStructName: oteltef.MetricsStructName},
		Callbacks: stefgrpc.ClientCallbacks{
			OnAck: func(ackId uint64) error { return conn.onGrpcAck(ackId) },
		},
	}
	conn.client, err = stefgrpc.NewClient(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create STEF client: %w", err)
	}

	connCtx, connCancel := context.WithCancel(context.Background())

	connectionAttemptDone := make(chan struct{})
	defer close(connectionAttemptDone)

	// Start a goroutine that waits for success, failure or cancellation of
	// the connection attempt.
	go func() {
		// Wait for either connection attempt to be done or for the caller
		// of Create() to give up.
		select {
		case <-ctx.Done():
			// The caller of Create() cancelled while we are waiting
			// for connection to be established. We have to cancel the
			// connection attempt (and the whole connection if it raced us and
			// managed to connect - we will reconnect later again in that case).
			s.logger.Debug("Canceling connection context because Create() caller cancelled.")
			connCancel()
		case <-connectionAttemptDone:
			// Connection attempt finished (successfully or no). No need to wait for the
			// previous case, calling connCancel() is not needed anymore now. It will be
			// called later, when disconnecting.
			// From this moment we are essentially detaching from the Context
			// that passed to Create() since we wanted to honor it only
			// for the duration of the connection attempt, but not for the duration
			// of the entire existence of the connection.
		}
	}()

	grpcWriter, opts, err := conn.client.Connect(connCtx)
	if err != nil {
		connCancel()
		return nil, fmt.Errorf("failed to connect to destination: %w", err)
	}

	opts.Compression = s.compression

	// Create STEF record writer over gRPC.
	conn.writer, err = oteltef.NewMetricsWriter(grpcWriter, opts)
	if err != nil {
		connCancel()
		return nil, err
	}

	// We need to call the cancel func when this connection is over so that we don't
	// leak the Context we just created. This will be done in disconnect().
	conn.cancel = connCancel

	// Run flusher in a separate goroutine.
	go conn.flusher()

	s.logger.Debug("Connected to destination", zap.String("target", s.grpcConn.CanonicalTarget()))

	return conn, nil
}

// Writer returns the metrics writer that exists over this connection.
func (s *StefConn) Writer() *oteltef.MetricsWriter {
	return s.writer
}

// OnAck registers to notify via ackCh when the acknowledgment with
// the given ackID is received over this connection. When acknowledgment
// with the specified ackID is received, the AsyncResult with ackID
// will send to ackCh.
func (s *StefConn) OnAck(ackID uint64, ackCh chan<- AsyncResult) {
	s.mux.Lock()
	s.pendingAcks[DataID(ackID)] = ackCh
	s.mux.Unlock()
}

// onGrpcAck is called by stefgrpc.Client when an acknowledgment is received.
func (s *StefConn) onGrpcAck(ackID uint64) error {
	s.mux.Lock()
	// Notify all pending acks that have ackID smaller or equal to the received ackID.
	for pendingAckID, ch := range s.pendingAcks {
		if uint64(pendingAckID) <= ackID {
			delete(s.pendingAcks, pendingAckID)
			ch <- AsyncResult{DataID: pendingAckID}
		}
	}
	s.mux.Unlock()
	return nil
}

// Close the connection.
func (s *StefConn) Close(ctx context.Context) error {
	s.cancel()

	// Stop flusher goroutine
	close(s.flushReqCh)

	return s.client.Disconnect(ctx)
}

func (s *StefConn) flusher() {
	for range s.flushReqCh {
		s.flushResCh <- s.writer.Flush()
	}
}

// Flush any pending data over the connection.
func (s *StefConn) Flush(ctx context.Context) error {
	// Request a flush.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.flushReqCh <- struct{}{}:
	}

	// Wait until flushed.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-s.flushResCh:
		return err
	}
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
