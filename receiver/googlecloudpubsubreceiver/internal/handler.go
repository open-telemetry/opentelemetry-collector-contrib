// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

// Time to wait before restarting, when the stream stopped
const streamRecoveryBackoffPeriod = 250 * time.Millisecond

type StreamHandler struct {
	stream      pubsubpb.Subscriber_StreamingPullClient
	pushMessage func(ctx context.Context, message *pubsubpb.ReceivedMessage) error
	acks        []string
	mutex       sync.Mutex
	client      SubscriberClient

	clientID     string
	subscription string

	cancel context.CancelFunc
	// wait group for the send/receive function
	streamWaitGroup sync.WaitGroup
	// wait group for the handler
	handlerWaitGroup sync.WaitGroup
	settings         receiver.Settings
	telemetryBuilder *metadata.TelemetryBuilder
	// time that acknowledge loop waits before acknowledging messages
	ackBatchWait time.Duration

	isRunning atomic.Bool
}

func (handler *StreamHandler) ack(ackID string) {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	handler.acks = append(handler.acks, ackID)
}

func NewHandler(
	ctx context.Context,
	settings receiver.Settings,
	telemetryBuilder *metadata.TelemetryBuilder,
	client SubscriberClient,
	clientID string,
	subscription string,
	callback func(ctx context.Context, message *pubsubpb.ReceivedMessage) error,
) (*StreamHandler, error) {
	handler := StreamHandler{
		settings:         settings,
		telemetryBuilder: telemetryBuilder,
		client:           client,
		clientID:         clientID,
		subscription:     subscription,
		pushMessage:      callback,
		ackBatchWait:     10 * time.Second,
	}
	return &handler, handler.initStream(ctx)
}

func (handler *StreamHandler) initStream(ctx context.Context) error {
	var err error
	// Create a stream, but with the receivers context as we don't want to cancel and ongoing operation
	handler.stream, err = handler.client.StreamingPull(ctx)
	if err != nil {
		return err
	}

	request := pubsubpb.StreamingPullRequest{
		Subscription:             handler.subscription,
		StreamAckDeadlineSeconds: 60,
		ClientId:                 handler.clientID,
	}
	if err := handler.stream.Send(&request); err != nil {
		_ = handler.stream.CloseSend()
		return err
	}
	handler.telemetryBuilder.ReceiverGooglecloudpubsubStreamRestarts.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("otelcol.component.kind", "receiver"),
			attribute.String("otelcol.component.id", handler.settings.ID.String()),
		))
	return nil
}

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
			}
		}
		handler.settings.Logger.Debug("End of recovery loop, restarting.")
		time.Sleep(streamRecoveryBackoffPeriod)
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

func (handler *StreamHandler) acknowledgeMessages() error {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	if len(handler.acks) == 0 {
		return nil
	}
	request := pubsubpb.StreamingPullRequest{
		AckIds: handler.acks,
	}
	handler.acks = nil
	return handler.stream.Send(&request)
}

func (handler *StreamHandler) requestStream(ctx context.Context, cancel context.CancelFunc) {
	timer := time.NewTimer(handler.ackBatchWait)
	for {
		if err := handler.acknowledgeMessages(); err != nil {
			if errors.Is(err, io.EOF) {
				handler.settings.Logger.Warn("EOF reached")
				break
			}
			handler.settings.Logger.Error(fmt.Sprintf("Failed in acknowledge messages with error %v", err))
			break
		}
		select {
		case <-ctx.Done():
			handler.settings.Logger.Debug("requestStream <-ctx.Done()")
		case <-timer.C:
			timer.Reset(handler.ackBatchWait)
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			_ = handler.acknowledgeMessages()
			timer.Stop()
			break
		}
	}
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
				if err == nil {
					// When sending a message though the pipeline fails, we ignore the error. We'll let Pubsub
					// handle the flow control.
					handler.ack(message.AckId)
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
