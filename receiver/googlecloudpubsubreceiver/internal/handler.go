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

// ErrNack is the sentinel error the pushMessage callback returns to negatively
// acknowledge a message (modify ack deadline to 0) instead of acknowledging it
// or leaving it to expire.
var ErrNack = errors.New("nack pubsub message")

type StreamHandler struct {
	stream      pubsubpb.Subscriber_StreamingPullClient
	pushMessage func(ctx context.Context, message *pubsubpb.ReceivedMessage) error
	acks        []string
	nacks       []string
	// ackFlush signals the requestStream loop to flush pending acks/nacks early
	ackFlush chan struct{}
	mutex    sync.Mutex
	client   SubscriberClient

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
	restartCount int
}

// ack adds the ackID to the list of message to be acknowledged asynchronously
func (handler *StreamHandler) ack(ackID string) {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	handler.acks = append(handler.acks, ackID)
	handler.signalFlushIfFull()
}

// nack adds the ackID to the list of messages to be negatively acknowledged
// (ack deadline modified to 0) asynchronously
func (handler *StreamHandler) nack(ackID string) {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	handler.nacks = append(handler.nacks, ackID)
	handler.signalFlushIfFull()
}

// signalFlushIfFull wakes the requestStream loop when enough acknowledgements
// are pending, so a stream break doesn't redeliver a full timer window worth of
// already processed messages. Callers must hold the mutex.
func (handler *StreamHandler) signalFlushIfFull() {
	batchSize := handler.flowControlConfig.TriggerAckBatchSize
	if batchSize <= 0 {
		// 0 (or negative) means the size trigger is disabled
		return
	}
	if len(handler.acks)+len(handler.nacks) >= batchSize {
		select {
		case handler.ackFlush <- struct{}{}:
		default:
		}
	}
}

// zeroDeadlines returns the ModifyDeadlineSeconds values (all 0, a nack) that
// pair with n ModifyDeadlineAckIds.
func zeroDeadlines(n int) []int32 {
	if n == 0 {
		return nil
	}
	return make([]int32, n)
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
		ackFlush:          make(chan struct{}, 1),
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

	handler.mutex.Lock()
	request := pubsubpb.StreamingPullRequest{
		Subscription:             handler.subscription,
		StreamAckDeadlineSeconds: int32(handler.flowControlConfig.StreamAckDeadline.Seconds()),
		ClientId:                 handler.clientID,
		MaxOutstandingMessages:   handler.flowControlConfig.MaxOutstandingMessages,
		MaxOutstandingBytes:      handler.flowControlConfig.MaxOutstandingBytes,
		AckIds:                   handler.acks,
		ModifyDeadlineAckIds:     handler.nacks,
		ModifyDeadlineSeconds:    zeroDeadlines(len(handler.nacks)),
	}
	if err := handler.stream.Send(&request); err != nil {
		handler.mutex.Unlock()
		_ = handler.stream.CloseSend()
		return err
	}
	handler.acks = nil
	handler.nacks = nil
	handler.mutex.Unlock()
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
				handler.settings.Logger.Error("Failed to recover stream.", zap.Error(err))
				handler.retryAttempt++
			} else {
				handler.retryAttempt = 0
			}
			handler.restartCount++
			handler.settings.Logger.Info("Restarting Pub/Sub stream.",
				zap.Int("restart_count", handler.restartCount),
				zap.Int("retry_attempt", handler.retryAttempt))
		}
		handler.settings.Logger.Debug("End of recovery loop, restarting.")
		time.Sleep(exponentialBackoff(handler.retryAttempt))
	}
	handler.settings.Logger.Warn("Shutting down recovery loop.")
	handler.handlerWaitGroup.Done()
}

func (handler *StreamHandler) CancelNow() {
	handler.isRunning.Swap(false)
	// Flush what is pending before tearing down the stream, and again after the
	// stream goroutines finished, for messages processed during shutdown. Without
	// this, the last batch of acknowledgements rides a stream whose context is
	// already cancelled and is lost, redelivering already processed messages.
	handler.finalFlush()
	if handler.cancel != nil {
		handler.cancel()
		handler.Wait()
	}
	handler.finalFlush()
}

// finalFlush acknowledges pending acks/nacks over the unary RPCs with a detached
// context, as the stream (and its context) is going away at shutdown.
func (handler *StreamHandler) finalFlush() {
	handler.mutex.Lock()
	acks := handler.acks
	nacks := handler.nacks
	handler.acks = nil
	handler.nacks = nil
	handler.mutex.Unlock()
	if len(acks) == 0 && len(nacks) == 0 {
		return
	}
	// detached context: the handler contexts are canceled (or canceling) at this point
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if len(acks) > 0 {
		if err := handler.client.Acknowledge(ctx, &pubsubpb.AcknowledgeRequest{
			Subscription: handler.subscription,
			AckIds:       acks,
		}); err != nil {
			handler.settings.Logger.Warn("failed to acknowledge messages on shutdown, they will be redelivered",
				zap.Int("count", len(acks)), zap.Error(err))
		}
	}
	if len(nacks) > 0 {
		if err := handler.client.ModifyAckDeadline(ctx, &pubsubpb.ModifyAckDeadlineRequest{
			Subscription:       handler.subscription,
			AckIds:             nacks,
			AckDeadlineSeconds: 0,
		}); err != nil {
			handler.settings.Logger.Warn("failed to nack messages on shutdown, they will be redelivered after the ack deadline",
				zap.Int("count", len(nacks)), zap.Error(err))
		}
	}
}

func (handler *StreamHandler) Wait() {
	handler.handlerWaitGroup.Wait()
}

// acknowledgeMessages will acknowledge the messages, and only clear the outstanding messages when the
// acknowledgement is send successfully
func (handler *StreamHandler) acknowledgeMessages() error {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	if len(handler.acks) == 0 && len(handler.nacks) == 0 {
		return nil
	}
	request := pubsubpb.StreamingPullRequest{
		AckIds:                handler.acks,
		ModifyDeadlineAckIds:  handler.nacks,
		ModifyDeadlineSeconds: zeroDeadlines(len(handler.nacks)),
	}
	err := handler.stream.Send(&request)
	if err == nil {
		handler.acks = nil
		handler.nacks = nil
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
		case <-handler.ackFlush:
			handler.settings.Logger.Debug("requestStream size-triggered acknowledge flush")
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
	for activeStreaming {
		// block until the next message or timeout expires
		resp, err := handler.stream.Recv()
		if err == nil {
			for _, message := range resp.ReceivedMessages {
				// handle all the messages in the response, could be one or more
				err = handler.pushMessage(context.Background(), message)
				switch {
				case err == nil:
					handler.ack(message.AckId)
				case errors.Is(err, context.Canceled):
					// The collector is probably shutting down, don't ack nor nack so the
					// message is redelivered and processed by a healthy subscriber.
				case errors.Is(err, ErrNack):
					handler.nack(message.AckId)
				default:
					// The message is neither acked nor nacked and will be redelivered
					// after the ack deadline expires.
				}
			}
		} else {
			s, grpcStatus := status.FromError(err)
			switch {
			case errors.Is(err, io.EOF):
				handler.settings.Logger.Info("response stream closed by the server (EOF), restarting stream",
					zap.Error(err))
				activeStreaming = false
			case !grpcStatus:
				handler.settings.Logger.Warn("response stream breaking on error",
					zap.Error(err))
				activeStreaming = false
			case s.Code() == codes.Unavailable:
				handler.settings.Logger.Info("response stream breaking on gRPC status 'Unavailable', restarting stream",
					zap.Error(err))
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
