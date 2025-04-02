// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver/internal"

import (
	"sync/atomic"
	"time"

	stefgrpc "github.com/splunk/stef/go/grpc"
	"github.com/splunk/stef/go/grpc/stef_proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Responder is responsible for sending responses to the STEF/gRPC client.
// There is one Responder per stream.
type Responder struct {
	logger *zap.Logger

	// Stream to send responses to.
	stream stefgrpc.STEFStream

	// Channel to stop the Responder goroutine.
	stopCh chan struct{}

	// The next ack ID to send to the client.
	nextAckID atomic.Uint64

	// The last error that occurred while sending responses.
	lastError atomic.Value

	// Channel to receive bad data information from. Data written to this channel
	// will be sent as a response to the client.
	badDataCh chan BadData

	// Interval at which the responder sends acks.
	ackInterval time.Duration
}

// BadData describes a range of records that were bad.
type BadData struct {
	// The range of records that were bad. FromID<=ToID.
	// ToID is also equal to the last ID read from STEF stream.
	FromID, ToID uint64
}

// Max number of BadData records that can be accumulated and be sent in a single response.
// The most typical number in one response will be 1, larger numbers are very unlikely, so we
// don't need a large buffer.
const badDataMaxBatchSize = 10

func NewResponder(logger *zap.Logger, stream stefgrpc.STEFStream, ackInterval time.Duration) *Responder {
	return &Responder{
		logger:      logger,
		stream:      stream,
		stopCh:      make(chan struct{}),
		badDataCh:   make(chan BadData, badDataMaxBatchSize),
		ackInterval: ackInterval,
	}
}

// ScheduleAck schedules an ack to be sent to the client. The ack will be sent
// at the next opportunity in a response to client. If ScheduleAck is called
// repeatedly, only the last value will be used in the next response.
// It is normally expected that ScheduleAck is called with increasing recordID values.
func (r *Responder) ScheduleAck(recordID uint64) {
	r.nextAckID.Store(recordID)
}

// ScheduleBadDataResponse schedules BadData record to be sent to the client.
// Multiple successive calls may be accumulated in a batch and sent in
// one response to the client.
func (r *Responder) ScheduleBadDataResponse(badData BadData) {
	r.badDataCh <- badData
}

// LastError returns the last error that occurred while sending responses or
// nil if there never was an error.
func (r *Responder) LastError() error {
	err := r.lastError.Load()
	if err == nil {
		return nil
	}
	return err.(error)
}

// Stop begins stopping the Responder.
func (r *Responder) Stop() {
	close(r.stopCh)
}

// Run the Responder. Normally called on a separate goroutine and blocks
// until stopped by calling Stop().
func (r *Responder) Run() {
	// Time interval to wait before sending an ack.
	t := time.NewTicker(r.ackInterval)
	defer t.Stop()

	lastAckedID := &atomic.Uint64{}

	// Preallocate to avoid allocations in the loop.
	badDataResponse := &stef_proto.STEFDataResponse{
		BadDataRecordIdRanges: make([]*stef_proto.STEFIDRange, 0, 8),
	}
	ackResponse := &stef_proto.STEFDataResponse{}

	for {
		select {
		case badData := <-r.badDataCh:
			r.composeBadDataResponse(badDataResponse, badData)
			if err := r.stream.SendDataResponse(badDataResponse); err != nil {
				r.logger.Error("Error acking STEF gRPC connection", zap.Error(err))
				r.lastError.Store(err)
			} else {
				lastAckedID.Store(badDataResponse.AckRecordId)
			}

		case <-t.C:
			readRecordID := r.nextAckID.Load()
			oldValue := lastAckedID.Swap(readRecordID)
			if readRecordID > oldValue {
				ackResponse.AckRecordId = readRecordID
				if err := r.stream.SendDataResponse(ackResponse); err != nil {
					st, ok := status.FromError(err)
					// This is not a regular disconnection case, as in the client closed the connection.
					if !ok || st.Code() != codes.Canceled {
						r.logger.Error("Error acking STEF gRPC connection", zap.Error(err))
						r.lastError.Store(err)
					}
				}
			}

		case <-r.stopCh:
			return
		}
	}
}

func (r *Responder) composeBadDataResponse(response *stef_proto.STEFDataResponse, badData BadData) {
	response.AckRecordId = badData.ToID

	// First bad data range.
	response.BadDataRecordIdRanges = response.BadDataRecordIdRanges[:1]
	response.BadDataRecordIdRanges[0].FromId = badData.FromID
	response.BadDataRecordIdRanges[0].ToId = badData.ToID

	// See if there is more bad data we can report all of it in the same response.
	for {
		select {
		case moreBadData := <-r.badDataCh:
			// Add a range.
			response.BadDataRecordIdRanges = append(
				response.BadDataRecordIdRanges, &stef_proto.STEFIDRange{
					FromId: moreBadData.FromID,
					ToId:   moreBadData.ToID,
				},
			)

			// Use the last ID value for AckRecordID
			if response.AckRecordId < moreBadData.ToID {
				response.AckRecordId = moreBadData.ToID
			}
		default:
			return
		}
	}
}
