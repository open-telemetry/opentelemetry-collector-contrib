// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

type StreamHandler struct {
	stream      pubsubpb.Subscriber_StreamingPullClient
	pushMessage func(ctx context.Context, message *pubsubpb.ReceivedMessage) error
	acks        []string
	// modAckDeadlines and modAckIDs are paired slices: modAckIDs[i] gets deadline modAckDeadlines[i].
	modAckDeadlines []int32
	modAckIDs       []string
	mutex           sync.Mutex
	client          SubscriberClient

	clientID     string
	subscription string

	cancel context.CancelFunc
	// wait group for the send/receive function
	streamWaitGroup sync.WaitGroup
	// wait group for the handler
	handlerWaitGroup sync.WaitGroup
	settings         receiver.Settings
	telemetryBuilder *metadata.TelemetryBuilder

	// flow control settings, like max durations, counts and triggers
	flowControlConfig *FlowControlConfig

	isRunning    atomic.Bool
	retryAttempt int
}

// ack adds the ackID to the list of messages to be acknowledged asynchronously.
func (handler *StreamHandler) ack(ackID string) {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	handler.acks = append(handler.acks, ackID)
}

// modifyAckDeadline queues a deadline modification for the given ackID.
// Use the configured ModAckDeadlineSeconds to extend processing time, or 0 to nack.
func (handler *StreamHandler) modifyAckDeadline(ackID string, deadlineSeconds int32) {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	handler.modAckIDs = append(handler.modAckIDs, ackID)
	handler.modAckDeadlines = append(handler.modAckDeadlines, deadlineSeconds)
}

// nack sends a ModifyAckDeadline with deadline 0, making the message immediately
// available for redelivery. This allows Pub/Sub to increment the delivery attempt
// counter so messages eventually reach the dead-letter queue.
func (handler *StreamHandler) nack(ackID string) {
	handler.modifyAckDeadline(ackID, 0)
}

func NewHandler(
	ctx context.Context,
	settings receiver.Settings,
	telemetryBuilder *metadata.TelemetryBuilder,
	client SubscriberClient,
	clientID string,
	subscription string,
	config *FlowControlConfig,
	callback func(ctx context.Context, message *pubsubpb.ReceivedMessage) error,
) (*StreamHandler, error) {
	if config == nil {
		config = NewDefaultFlowControlConfig()
	}
	handler := StreamHandler{
		settings:          settings,
		telemetryBuilder:  telemetryBuilder,
		client:            client,
		clientID:          clientID,
		subscription:      subscription,
		pushMessage:       callback,
		flowControlConfig: config,
	}
	return &handler, handler.initStream(ctx)
}

// initStream creates a new streaming pull stream. When the previous stream was closed, the
// pending acknowledge messages will be acknowledged at stream re-creation.
func (handler *StreamHandler) initStream(ctx context.Context) error {
	var err error
	// Create a stream, but with the receivers context as we don't want to cancel and ongoing operation
	handler.stream, err = handler.client.StreamingPull(ctx)
	if err != nil {
		return err
	}

	request := pubsubpb.StreamingPullRequest{
		Subscription:             handler.subscription,
		StreamAckDeadlineSeconds: int32(handler.flowControlConfig.StreamAckDeadline.Seconds()),
		ClientId:                 handler.clientID,
		MaxOutstandingMessages:   handler.flowControlConfig.MaxOutstandingMessages,
		MaxOutstandingBytes:      handler.flowControlConfig.MaxOutstandingBytes,
		AckIds:                   handler.acks,
		ModifyDeadlineAckIds:     handler.modAckIDs,
		ModifyDeadlineSeconds:    handler.modAckDeadlines,
	}
	if err := handler.stream.Send(&request); err != nil {
		_ = handler.stream.CloseSend()
		return err
	}
	handler.acks = nil
	handler.modAckIDs = nil
	handler.modAckDeadlines = nil
	handler.telemetryBuilder.ReceiverGooglecloudpubsubStreamRestarts.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("otelcol.component.kind", "receiver"),
			attribute.String("otelcol.component.id", handler.settings.ID.String()),
		))
	return nil
}

// RecoverableStream starts the Pub/Sub stream loop and recovers it if it fails
func (handler *StreamHandler) RecoverableStream(ctx context.Context) {
	handler.handlerWaitGroup.Add(1)
	handler.isRunning.Swap(true)
	var handlerCtx context.Context
	handlerCtx, handler.cancel = context.WithCancel(ctx)
	go handler.recoverableStream(handlerCtx)
}

func (handler *StreamHandler) recoverableStream(ctx context.Context) {
	for handler.isRunning.Load() {
		// Create a new cancelable context for the handler, so we can recover the stream
		var loopCtx context.Context
		loopCtx, cancel := context.WithCancel(ctx)

		handler.settings.Logger.Debug("Starting Streaming Pull")
		handler.streamWaitGroup.Add(2)
		go handler.requestStream(loopCtx, cancel)
		go handler.responseStream(loopCtx, cancel)

		select {
		case <-loopCtx.Done():
			handler.streamWaitGroup.Wait()
		case <-ctx.Done():
			cancel()
			handler.streamWaitGroup.Wait()
		}
		if handler.isRunning.Load() {
			err := handler.initStream(ctx)
			if err != nil {
				handler.settings.Logger.Error("Failed to recovery stream.")
				handler.retryAttempt++
			} else {
				handler.retryAttempt = 0
			}
		}
		handler.settings.Logger.Debug("End of recovery loop, restarting.")
		time.Sleep(exponentialBackoff(handler.retryAttempt))
	}
	handler.settings.Logger.Warn("Shutting down recovery loop.")
	handler.handlerWaitGroup.Done()
}

func (handler *StreamHandler) CancelNow() {
	handler.isRunning.Swap(false)
	if handler.cancel != nil {
		handler.cancel()
		handler.Wait()
	}
}

func (handler *StreamHandler) Wait() {
	handler.handlerWaitGroup.Wait()
}

// acknowledgeMessages sends pending acks and ModifyAckDeadline requests.
// Pending lists are only cleared on successful send so they can be retried
// on the next stream if the current one breaks.
func (handler *StreamHandler) acknowledgeMessages() error {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	if len(handler.acks) == 0 && len(handler.modAckIDs) == 0 {
		return nil
	}
	request := pubsubpb.StreamingPullRequest{
		AckIds:                 handler.acks,
		ModifyDeadlineAckIds:   handler.modAckIDs,
		ModifyDeadlineSeconds:  handler.modAckDeadlines,
	}
	err := handler.stream.Send(&request)
	if err == nil {
		handler.acks = nil
		handler.modAckIDs = nil
		handler.modAckDeadlines = nil
	}
	return err
}

// requestStream waits for triggers to acknowledge messages that have been processed by the collector. If
// a stream got restarted, the messages that still needed to be acknowledged are acknowledged at the start
// of the new stream, so we don't need to start with an acknowledgeMessages.
func (handler *StreamHandler) requestStream(ctx context.Context, cancel context.CancelFunc) {
	timer := time.NewTimer(handler.flowControlConfig.TriggerAckBatchDuration)
	for {
		select {
		case <-ctx.Done():
			handler.settings.Logger.Debug("requestStream <-ctx.Done()")
		case <-timer.C:
		}
		// whatever happens, we need to acknowledge the messages
		if err := handler.acknowledgeMessages(); err != nil {
			if errors.Is(err, io.EOF) {
				handler.settings.Logger.Warn("EOF reached")
				break
			}
			handler.settings.Logger.Error(fmt.Sprintf("Failed in acknowledge messages with error %v", err))
			break
		}
		// if the context is canceled, we break the loop
		if errors.Is(ctx.Err(), context.Canceled) {
			break
		}
		timer.Reset(handler.flowControlConfig.TriggerAckBatchDuration)
	}
	timer.Stop()
	cancel()
	handler.settings.Logger.Debug("Request Stream loop ended.")
	_ = handler.stream.CloseSend()
	handler.streamWaitGroup.Done()
}

func (handler *StreamHandler) responseStream(ctx context.Context, cancel context.CancelFunc) {
	activeStreaming := true
	modAckDeadline := int32(handler.flowControlConfig.ModAckDeadlineSeconds)
	for activeStreaming {
		// block until the next message or timeout expires
		resp, err := handler.stream.Recv()
		if err == nil {
			for _, message := range resp.ReceivedMessages {
				// Immediately extend the ack deadline so Pub/Sub knows we are
				// actively processing this message. This mirrors the behavior
				// of the official GCP client libraries and is required for the
				// dead-letter queue delivery-attempt counter to work correctly
				// with StreamingPull.
				if modAckDeadline > 0 {
					handler.modifyAckDeadline(message.AckId, modAckDeadline)
				}

				err = handler.pushMessage(context.Background(), message)
				if err == nil {
					handler.ack(message.AckId)
				} else {
					// Nack the message so it becomes immediately available for
					// redelivery and the delivery attempt counter increments.
					handler.nack(message.AckId)
				}
			}
		} else {
			s, grpcStatus := status.FromError(err)
			switch {
			case errors.Is(err, io.EOF):
				activeStreaming = false
			case !grpcStatus:
				handler.settings.Logger.Warn("response stream breaking on error",
					zap.Error(err))
				activeStreaming = false
			case s.Code() == codes.Unavailable:
				handler.settings.Logger.Debug("response stream breaking on gRPC s 'Unavailable'")
				activeStreaming = false
			case s.Code() == codes.NotFound:
				handler.settings.Logger.Error("resource doesn't exist, wait 60 seconds, and restarting stream")
				time.Sleep(time.Second * 60)
				activeStreaming = false
			default:
				handler.settings.Logger.Warn("response stream breaking on gRPC s "+s.Message(),
					zap.String("s", s.Message()),
					zap.Error(err))
				activeStreaming = false
			}
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			// Canceling the loop, collector is probably stopping
			handler.settings.Logger.Warn("response stream ctx.Err() == context.Canceled")
			break
		}
	}
	cancel()
	handler.settings.Logger.Debug("Response Stream loop ended.")
	handler.streamWaitGroup.Done()
}

// exponentialBackoff will backoff exponentially with a maximum of 2 minutes
func exponentialBackoff(retryAttempt int) time.Duration {
	if retryAttempt < 1 {
		return 0
	}
	maxDuration := 2 * time.Minute
	backoffMs := 250.0 * math.Pow(2, float64(retryAttempt-1))
	if backoffMs > float64(maxDuration.Milliseconds()) {
		backoffMs = float64(maxDuration.Milliseconds())
	}
	return time.Duration(backoffMs*(0.7+rand.Float64()*0.3)) * time.Millisecond
}
