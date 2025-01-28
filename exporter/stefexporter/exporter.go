// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	stef_grpc "github.com/splunk/stef/go/grpc"
	"github.com/splunk/stef/go/grpc/stef_proto"
	"github.com/splunk/stef/go/otel/oteltef"
	stefpdatametrics "github.com/splunk/stef/go/pdata/metrics"
	"github.com/splunk/stef/go/pkg"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type stefExporter struct {
	set    component.TelemetrySettings
	host   component.Host
	logger *zap.Logger
	cfg    *Config

	connectionMutex sync.Mutex
	isConnected     bool
	grpcConn        *grpc.ClientConn
	compression     pkg.Compression

	maxWritesInProgress int64
	writesInProgress    int64
	writeMutex          sync.Mutex // protects remoteWriter
	remoteWriter        *oteltef.MetricsWriter

	lastAckedID      uint64
	lastAckedIDMutex sync.Mutex
	ackCond          *sync.Cond
	interruptAckWait bool

	stopFlusher chan struct{}
}

type grpcLogger struct {
	logger *zap.Logger
}

func (w *grpcLogger) Debugf(_ context.Context, format string, v ...any) {
	w.logger.Debug(fmt.Sprintf(format, v...))
}

func (w *grpcLogger) Errorf(_ context.Context, format string, v ...any) {
	w.logger.Error(fmt.Sprintf(format, v...))
}

var errAckTimeout = errors.New("ack timed out")

func newStefExporter(set component.TelemetrySettings, cfg *Config) *stefExporter {
	exp := &stefExporter{
		set:                 set,
		logger:              set.Logger,
		cfg:                 cfg,
		maxWritesInProgress: int64(cfg.NumConsumers),
	}

	exp.ackCond = sync.NewCond(&exp.lastAckedIDMutex)

	exp.compression = pkg.CompressionNone
	if cfg.Compression == "zstd" {
		exp.compression = pkg.CompressionZstd
	}
	return exp
}

func (s *stefExporter) Start(ctx context.Context, host component.Host) error {
	s.host = host

	// No need to block Start(), we will begin connection attempt in a goroutine.
	go func() {
		if err := s.ensureConnected(ctx, host); err != nil {
			s.logger.Error("Error connecting to destination", zap.Error(err))
			// pushMetrics() will try to connect again as needed.
		}
	}()
	return nil
}

func (s *stefExporter) ensureConnected(ctx context.Context, host component.Host) error {
	s.connectionMutex.Lock()
	defer s.connectionMutex.Unlock()

	if s.isConnected {
		return nil
	}

	s.lastAckedID = 0
	s.interruptAckWait = false

	s.logger.Debug("Connecting to destination", zap.String("endpoint", s.cfg.Endpoint))

	// Connect to the server.
	var err error
	s.grpcConn, err = s.cfg.ClientConfig.ToClientConn(ctx, host, s.set)
	if err != nil {
		return err
	}

	// Open a stream to the server.
	grpcClient := stef_proto.NewSTEFDestinationClient(s.grpcConn)

	schema, err := oteltef.MetricsWireSchema()
	if err != nil {
		return err
	}

	settings := stef_grpc.ClientSettings{
		Logger:       &grpcLogger{s.logger},
		GrpcClient:   grpcClient,
		ClientSchema: schema,
		Callbacks: stef_grpc.ClientCallbacks{
			OnAck: s.onGrpcAck,
		},
	}
	client := stef_grpc.NewClient(settings)

	grpcWriter, opts, err := client.Connect(context.Background())
	if err != nil {
		s.grpcConn.Close()
		return err
	}

	opts.Compression = s.compression

	// Create record writer.
	s.remoteWriter, err = oteltef.NewMetricsWriter(grpcWriter, opts)
	if err != nil {
		return err
	}

	s.stopFlusher = make(chan struct{})
	go s.flusher()

	s.isConnected = true

	return nil
}

func (s *stefExporter) disconnect() {
	s.connectionMutex.Lock()
	defer s.connectionMutex.Unlock()

	if !s.isConnected {
		return
	}

	s.logger.Debug("Disconnecting...")

	close(s.stopFlusher)

	if s.grpcConn != nil {
		if err := s.grpcConn.Close(); err != nil {
			s.logger.Error("failed to close grpc connection", zap.Error(err))
		}
	}

	s.isConnected = false
}

func (s *stefExporter) Shutdown(_ context.Context) error {
	s.disconnect()
	return nil
}

func (s *stefExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Keep count of how many concurrent writes are in progress.
	atomic.AddInt64(&s.writesInProgress, 1)
	defer atomic.AddInt64(&s.writesInProgress, -1)

	// Write the metrics to STEF stream.
	expectedAckID, err := s.writeMetrics(ctx, md)
	if err != nil {
		return err
	}

	// Wait for acknowledgement from destination.
	if err := s.waitForAck(expectedAckID); err != nil {
		return err
	}

	return nil
}

func (s *stefExporter) writeMetrics(ctx context.Context, md pmetric.Metrics) (uint64, error) {
	if err := s.ensureConnected(ctx, s.host); err != nil {
		return 0, err
	}

	// remoteWriter is not safe for concurrent writing.
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	converter := stefpdatametrics.OtlpToTEFUnsorted{}
	err := converter.WriteMetrics(md, s.remoteWriter)
	if err != nil {
		// TODO: check if err is because STEF encoding failed. If so we must not try to re-encode
		// the same data. Return consumererror.NewPermanent(err) to the caller.
		s.disconnect()

		// Return an error to retry sending these metrics again next time.
		return 0, err
	}

	// According to STEF gRPC spec the destination ack IDs match written record number.
	// When the data we have just written is received by destination it will send us
	// back and ack ID that numerically matches the last written record number.
	expectedAckID := s.remoteWriter.RecordCount()

	if s.writesInProgress >= s.maxWritesInProgress {
		// We achieved max concurrency. No further pushMetrics calls will make progress
		// until at least some of the pushMetrics calls return. For calls to return
		// they need to receive an Ack. For Ack to be received we need to make
		// the data we wrote is actually sent, so we need to issue a Flush call.
		if err = s.remoteWriter.Flush(); err != nil {
			// Failure to write the gRPC stream normally means something is wrong with the connection.
			// We will reconnect.
			s.disconnect()

			// Return an error to retry sending these metrics again next time.
			return 0, err
		}
	}

	return expectedAckID, nil
}

func (s *stefExporter) waitForAck(ackID uint64) error {
	s.ackCond.L.Lock()
	for s.lastAckedID < ackID && !s.interruptAckWait {
		s.ackCond.Wait()
	}
	s.ackCond.L.Unlock()

	if s.lastAckedID >= ackID {
		// We received the ack we were waiting for.
		return nil
	}

	s.logger.Error("Waiting for ack is interrupted.")

	return errAckTimeout
}

func (s *stefExporter) onGrpcAck(ackID uint64) error {
	s.lastAckedIDMutex.Lock()
	defer s.lastAckedIDMutex.Unlock()
	if s.lastAckedID < ackID {
		s.lastAckedID = ackID
		s.ackCond.Broadcast()
	}
	return nil
}

func (s *stefExporter) flusher() {
	timer := time.NewTicker(100 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			s.writeMutex.Lock()
			err := s.remoteWriter.Flush()
			s.writeMutex.Unlock()
			if err != nil {
				s.logger.Error("Error flushing data. Will disconnect to reconnect.", zap.Error(err))
				s.disconnect()

				s.logger.Debug("Interrupting any pending waitForAck() call. They can't succeed anymore.")
				s.lastAckedIDMutex.Lock()
				s.interruptAckWait = true
				s.ackCond.Broadcast()
				s.lastAckedIDMutex.Unlock()
				return
			}
		case <-s.stopFlusher:
			return
		}
	}
}
