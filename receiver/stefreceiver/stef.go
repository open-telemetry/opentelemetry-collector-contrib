// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver"
import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	stefgrpc "github.com/splunk/stef/go/grpc"
	"github.com/splunk/stef/go/grpc/stef_proto"
	"github.com/splunk/stef/go/otel/oteltef"
	stefpdatametrics "github.com/splunk/stef/go/pdata/metrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver/internal"
)

type stefReceiver struct {
	cfg        *Config
	serverGRPC *grpc.Server

	nextMetricsConsumer consumer.Metrics
	settings            receiver.Settings

	stopping atomic.Bool
	eg       errgroup.Group
}

// Start runs the STEF gRPC receiver.
func (r *stefReceiver) Start(ctx context.Context, host component.Host) error {
	r.stopping.Store(false)

	var err error
	if r.serverGRPC, err = r.cfg.ToServer(ctx, host, r.settings.TelemetrySettings); err != nil {
		return err
	}

	r.settings.Logger.Info("Starting GRPC server", zap.String("endpoint", r.cfg.NetAddr.Endpoint))

	schema, err := oteltef.MetricsWireSchema()
	if err != nil {
		return err
	}

	var gln net.Listener
	if gln, err = r.cfg.NetAddr.Listen(context.Background()); err != nil {
		return err
	}

	settings := stefgrpc.ServerSettings{
		Logger:       nil,
		ServerSchema: &schema,
		MaxDictBytes: 0,
		Callbacks:    stefgrpc.Callbacks{OnStream: r.onStream},
	}

	stefSrv := stefgrpc.NewStreamServer(settings)
	stef_proto.RegisterSTEFDestinationServer(r.serverGRPC, stefSrv)

	r.eg.Go(func() error {
		if errGrpc := r.serverGRPC.Serve(gln); errGrpc != nil && !errors.Is(errGrpc, grpc.ErrServerStopped) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errGrpc))
		}
		return nil
	})
	return nil
}

// Shutdown is a method to turn off receiving.
func (r *stefReceiver) Shutdown(_ context.Context) error {
	r.stopping.Store(true)

	if r.serverGRPC != nil {
		r.settings.Logger.Info("Stopping STEF/gRPC server", zap.String("endpoint", r.cfg.NetAddr.Endpoint))

		// Give graceful stop a second to finish.
		timer := time.AfterFunc(
			1*time.Second, func() {
				r.settings.Logger.Info("STEF/gRPC server couldn't stop gracefully in time. Doing force stop.")
				r.serverGRPC.Stop()
			},
		)
		defer timer.Stop()

		r.serverGRPC.GracefulStop()
		r.settings.Logger.Debug("STEF/gRPC server stopped.")
	}

	return r.eg.Wait()
}

func (r *stefReceiver) onStream(grpcReader stefgrpc.GrpcReader, stream stefgrpc.STEFStream) error {
	r.settings.Logger.Debug("Incoming STEF/gRPC connection.")

	reader, err := oteltef.NewMetricsReader(grpcReader)
	if err != nil {
		r.settings.Logger.Error("Cannot decode data on incoming STEF/gRPC connection", zap.Error(err))
		return err
	}

	// Create a responder for this stream and run it in a separate goroutine.
	resp := internal.NewResponder(r.settings.Logger, stream, r.cfg.AckInterval)
	defer resp.Stop()
	go resp.Run()

	converter := stefpdatametrics.STEFToOTLPUnsorted{}

	// Read, decode, convert the incoming data and push it to the next consumer.
	for {
		if r.stopping.Load() {
			// The receiver is shutting down. Close the connection.
			r.settings.Logger.Debug("Shutdown requested. Closing STEF/gRPC connection.")
			return nil
		}

		respError := resp.LastError()
		if respError != nil {
			// We had problem sending responses. Can't continue using this connection since
			// responding is essential for operation.
			r.settings.Logger.Error(
				"Closing STEF/gRPC connection since responding failed",
				zap.Error(respError),
			)
			return respError
		}

		// Mark the start of the converted batch.
		fromRecordID := reader.RecordCount()

		// Read and convert records. We use ConvertTillEndOfFrame to make sure we are not
		// blocked in the middle of a batch indefinitely, with lingering data in memory,
		// neither pushed to pipeline, nor acked.
		mdata, err := converter.ConvertTillEndOfFrame(reader)
		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Canceled {
				// A regular disconnection case. The client closed the connection.
				r.settings.Logger.Debug("STEF/gRPC connection closed", zap.Error(err))
			} else {
				r.settings.Logger.Error("Cannot read from STEF/gRPC connection", zap.Error(err))
			}
			return err
		}

		toRecordID := reader.RecordCount()

		// Push converted data to the next consumer.
		if err := r.nextMetricsConsumer.ConsumeMetrics(context.Background(), mdata); err != nil {
			r.settings.Logger.Error(
				"Error pushing data to consumer",
				zap.Error(err),
				zap.Uint64("fromID", fromRecordID),
				zap.Uint64("toID", toRecordID),
			)

			if !consumererror.IsPermanent(err) {
				// The next consumer is temporarily unable to process the data.
				// Close the stream and indicate to client to try again later.
				return status.New(codes.Unavailable, "try again later").Err()
			}
			// This is a permanent error. Let the client know and continue receiving data.
			resp.ScheduleBadDataResponse(
				internal.BadData{
					FromID: fromRecordID,
					ToID:   toRecordID,
				},
			)
		} else {
			// Successfully received and consumed. Acknowledge it.
			resp.ScheduleAck(toRecordID)
		}
	}
}
