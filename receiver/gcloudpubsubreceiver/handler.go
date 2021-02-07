// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcloudpubsubreceiver

import (
	"context"
	"fmt"
	pubsubpb "google.golang.org/genproto/googleapis/pubsub/v1"
	"io"
	"os"
	"sync"
	"time"
)

// https://cloud.google.com/pubsub/docs/reference/rpc/google.pubsub.v1#streamingpullrequest
type streamHandler struct {
	receiver    *pubsubReceiver
	stream      pubsubpb.Subscriber_StreamingPullClient
	pushMessage func(ctx context.Context, message *pubsubpb.ReceivedMessage) error
	acks        []string
	mutex       sync.Mutex

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (handler *streamHandler) ack(ackId string) {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	handler.acks = append(handler.acks, ackId)
}

// When sending a message though the pipeline fails, we ignore the error. We'll let Pubsub
// handle the flow control.
func (handler *streamHandler) nak(id string, err error) {
}

func (handler *streamHandler) initStream(ctx context.Context, subscription string) error {
	var err error
	// Create a stream, but with the receivers context as we don't want to cancel and ongoing operation
	handler.stream, err = handler.receiver.client.StreamingPull(ctx)
	if err != nil {
		return err
	}

	request := pubsubpb.StreamingPullRequest{
		Subscription:             subscription,
		StreamAckDeadlineSeconds: 60,
		ClientId:                 handler.receiver.config.ClientId + "-traces",
		// TODO: Revisit setting outstanding messages, how thus this inpact concurrency amongst pods
		//MaxOutstandingMessages:   20,
	}
	if err := handler.stream.Send(&request); err != nil {
		_ = handler.stream.CloseSend()
		return err
	}
	return nil
}

func (handler *streamHandler) recoverableStream(ctx context.Context, subscription string) {
	for {
		// Create a new cancelable context for the handler, so we can recover the stream
		var handlerCtx context.Context
		handlerCtx, handler.cancel = context.WithCancel(ctx)

		handler.receiver.logger.Info("Starting Streaming Pull")
		handler.wg.Add(2)
		go handler.requestStream(handlerCtx)
		go handler.responseStream(handlerCtx)

		select {
		case <-handlerCtx.Done():
			handler.wg.Wait()
		case <-ctx.Done():
		}
		if ctx.Err() == context.Canceled {
			handler.cancel()
			handler.wg.Wait()
			break
		} else {
			err := handler.initStream(ctx, subscription)
			if err != nil {
				// TODO, can't re-init, crash?!
				handler.receiver.logger.Error("Suicide")
				os.Exit(3)
			}
		}
		handler.receiver.logger.Warn("End of recovery loop, restarting.")
	}
	handler.receiver.logger.Warn("Shutting down recovery loop.")
	handler.receiver.wg.Done()
}

func (handler *streamHandler) acknowledgeMessages() error {
	handler.mutex.Lock()
	if len(handler.acks) == 0 {
		handler.mutex.Unlock()
		return nil
	}
	request := pubsubpb.StreamingPullRequest{
		AckIds: handler.acks,
	}
	handler.acks = make([]string, 0)
	handler.mutex.Unlock()
	return handler.stream.Send(&request)
}

func (handler *streamHandler) requestStream(ctx context.Context) {
	duration := 10000 * time.Millisecond
	timer := time.NewTimer(duration)
	for {
		if err := handler.acknowledgeMessages(); err != nil {
			if err == io.EOF {
				handler.receiver.logger.Warn("EOF reached")
				break
			}
			// TODO: When can we not ack messages?
			// TODO: For now we continue the loop and hope for the best
			fmt.Println("Failed in acknowledge messages with error", err)
			break
		}
		select {
		case <-ctx.Done():
			handler.receiver.logger.Warn("requestStream <-ctx.Done()")
		case <-timer.C:
			timer.Reset(duration)
		}
		if ctx.Err() == context.Canceled {
			_ = handler.acknowledgeMessages()
			timer.Stop()
			break
		}
	}
	handler.cancel()
	handler.receiver.logger.Warn("Request Stream loop ended.")
	_ = handler.stream.CloseSend()
	handler.wg.Done()
}

func (handler *streamHandler) responseStream(ctx context.Context) {
	for {
		// block until the next message or timeout expires
		resp, err := handler.stream.Recv()
		if err == nil {
			for _, message := range resp.ReceivedMessages {
				// handle all the messages in the response, could be one or more
				err := handler.pushMessage(context.Background(), message)
				if err != nil {
					handler.nak(message.AckId, err)
				}
				handler.ack(message.AckId)
			}
		} else {
			if err == io.EOF {
				break
			}
			// TODO: we're not going to quit the loop on an error,
			// TODO: though need to investigate if we need to break
			handler.receiver.logger.Warn("responseStream breaking on error")
			break
		}
		if ctx.Err() == context.Canceled {
			// Cancelling the loop, collector is probably stopping
			handler.receiver.logger.Warn("responseStream ctx.Err() == context.Canceled")
			break
		}
	}
	handler.cancel()
	handler.receiver.logger.Warn("Response Stream loop ended.")
	handler.wg.Done()
}
