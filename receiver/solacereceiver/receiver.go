// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// solaceTracesReceiver uses azure AMQP to consume and handle telemetry data from SOlace. Implements component.TracesReceiver
type solaceTracesReceiver struct {
	instanceID config.ComponentID
	// config is the receiver.Config instance used to build the receiver
	config *Config

	nextConsumer consumer.Traces
	settings     component.ReceiverCreateSettings
	unmarshaller tracesUnmarshaller
	// cancel is the function that will cancel the context associated with the main worker loop
	cancel            context.CancelFunc
	shutdownWaitGroup *sync.WaitGroup
	// newFactory is the constructor to use to build new messagingServiceFactory instances
	factory messagingServiceFactory
	// terminating is used to indicate that the receiver is terminating
	terminating *atomic.Bool
	// retryTimeout is the timeout between connection attempts
	retryTimeout time.Duration
}

// newTracesReceiver creates a new solaceTraceReceiver as a component.TracesReceiver
func newTracesReceiver(config *Config, receiverCreateSettings component.ReceiverCreateSettings, nextConsumer consumer.Traces) (component.TracesReceiver, error) {
	if nextConsumer == nil {
		receiverCreateSettings.Logger.Warn("Next consumer in pipeline is null, stopping receiver")
		return nil, component.ErrNilNextConsumer
	}

	if err := config.Validate(); err != nil {
		receiverCreateSettings.Logger.Warn("Error validating configuration", zap.Any("error", err))
		return nil, err
	}

	factory, err := newAMQPMessagingServiceFactory(config, receiverCreateSettings.Logger)
	if err != nil {
		receiverCreateSettings.Logger.Warn("Error validating messaging service configuration", zap.Any("error", err))
		return nil, err
	}

	unmarshaller := newTracesUnmarshaller(receiverCreateSettings.Logger)

	return &solaceTracesReceiver{
		instanceID:        config.ID(),
		config:            config,
		nextConsumer:      nextConsumer,
		settings:          receiverCreateSettings,
		unmarshaller:      unmarshaller,
		shutdownWaitGroup: &sync.WaitGroup{},
		factory:           factory,
		retryTimeout:      1 * time.Second,
		terminating:       atomic.NewBool(false),
	}, nil
}

// Start implements component.Receiver::Start
func (s *solaceTracesReceiver) Start(_ context.Context, _ component.Host) error {
	recordReceiverStatus(receiverStateStarting)
	var cancelableContext context.Context
	cancelableContext, s.cancel = context.WithCancel(context.Background())

	s.settings.Logger.Info("Starting receiver")
	// start the reconnection loop with a cancellable context and a factory to build new messaging services
	go s.connectAndReceive(cancelableContext)

	s.settings.Logger.Info("Receiver successfully started")
	return nil
}

// Shutdown implements component.Receiver::Shutdown
func (s *solaceTracesReceiver) Shutdown(ctx context.Context) error {
	s.terminating.Store(true)
	recordReceiverStatus(receiverStateTerminating)
	s.settings.Logger.Info("Shutdown waiting for all components to complete")
	s.cancel() // cancels the context passed to the reconneciton loop
	s.shutdownWaitGroup.Wait()
	s.settings.Logger.Info("Receiver shutdown successfully")
	recordReceiverStatus(receiverStateTerminated)
	return nil
}

func (s *solaceTracesReceiver) connectAndReceive(ctx context.Context) {
	// indicate we are started in the reconnection loop
	s.shutdownWaitGroup.Add(1)
	defer func() {
		s.settings.Logger.Info("Reconnection loop completed successfully")
		s.shutdownWaitGroup.Done()
	}()

	s.settings.Logger.Info("Starting reconnection and consume loop")
	disable := false

	// indicate we are in connecting state at the start
	recordReceiverStatus(receiverStateConnecting)

reconnectionLoop:
	for !disable {
		// check that we are not shutting down prior to the dial attempt
		select {
		case <-ctx.Done():
			s.settings.Logger.Debug("Received loop shutdown request")
			break reconnectionLoop
		default:
		}
		// create a new connection within the closure to defer the service.close
		func() {
			defer func() {
				// if the receiver is disabled, record the idle state, otherwise record the connecting state
				if disable {
					s.recordConnectionState(receiverStateIdle)
				} else {
					s.recordConnectionState(receiverStateConnecting)
				}
			}()
			service := s.factory()
			defer service.close(ctx)

			if err := service.dial(); err != nil {
				s.settings.Logger.Debug("Encountered error while connecting messaging service", zap.Error(err))
				recordFailedReconnection()
				return
			}
			// dial was successful, record the connected state
			s.recordConnectionState(receiverStateConnected)

			if err := s.receiveMessages(ctx, service); err != nil {
				s.settings.Logger.Debug("Encountered error while receiving messages", zap.Error(err))
				if errors.Is(err, errUnknownTraceMessgeVersion) {
					recordNeedUpgrade()
					disable = true
					return
				}
			}
		}()
		// sleep will be interrupted if ctx.Done() is closed
		sleep(ctx, s.retryTimeout)
	}
}

// recordConnectionState will record the given connection state unless in the terminating state.
// This does not fully prevent the state transitions terminating->(state)->terminated but
// is a best effort without mutex protection and additional state tracking, and in reality if
// this state transition were to happen, it would be short lived.
func (s *solaceTracesReceiver) recordConnectionState(state receiverState) {
	if !s.terminating.Load() {
		recordReceiverStatus(state)
	}
}

// receiveMessages will continuously receive, unmarshal and propagate messages
func (s *solaceTracesReceiver) receiveMessages(ctx context.Context, service messagingService) error {
	for {
		select { // ctx.Done will be closed when we should terminate
		case <-ctx.Done():
			return nil
		default:
		}
		// any error encountered will be returned to caller
		if err := s.receiveMessage(ctx, service); err != nil {
			return err
		}
	}

}

// receiveMessage is the heart of the receiver's control flow. It will receive messages, unmarshal the message and forward the trace.
// Will return an error if a fatal error occurs. It is expected that any error returned will cause a connection close.
func (s *solaceTracesReceiver) receiveMessage(ctx context.Context, service messagingService) (err error) {
	msg, err := service.receiveMessage(ctx)
	if err != nil {
		s.settings.Logger.Warn("Failed to receive message from messaging service", zap.Error(err))
		return err // propagate any receive message error up to caller
	}
	// only set the disposition action after we have received a message successfully
	disposition := service.ack
	defer func() { // on return of receiveMessage, we want to either ack or nack the message
		if actionErr := disposition(ctx, msg); err == nil && actionErr != nil {
			err = actionErr
		}
	}()
	// message received successfully
	recordReceivedSpanMessages()
	// unmarshal the message. unmarshalling errors are not fatal unless the version is unknown
	traces, unmarshalErr := s.unmarshaller.unmarshal(msg)
	if unmarshalErr != nil {
		s.settings.Logger.Error("Encountered error while unmarshalling message", zap.Error(unmarshalErr))
		recordFatalUnmarshallingError()
		if errors.Is(unmarshalErr, errUnknownTraceMessgeVersion) {
			disposition = service.nack // if we don't know the version, reject the trace message since we will disable the receiver
			return unmarshalErr
		}
		recordDroppedSpanMessages() // if the error is some other unmarshalling error, we will ack the message and drop the content
		return nil                  // don't propagate error, but don't continue forwarding traces
	}
	// forward to next consumer. Forwarding errors are not fatal so are not propagated to the caller.
	// Temporary consumer errors will lead to redelivered messages, permanent will be accepted
	forwardErr := s.nextConsumer.ConsumeTraces(ctx, traces)
	if forwardErr != nil {
		if !consumererror.IsPermanent(forwardErr) { // reject the message if the error is not permanent so we can retry, don't increment dropped span messages
			s.settings.Logger.Warn("Encountered temporary error while forwarding traces to next receiver, will allow redelivery", zap.Error(forwardErr))
			disposition = service.nack
		} else { // error is permanent, we want to accept the message and increment the number of dropped messages
			s.settings.Logger.Warn("Encountered permanent error while forwarding traces to next receiver, will swallow trace", zap.Error(forwardErr))
			recordDroppedSpanMessages()
		}
	} else {
		recordReportedSpans()
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		timer.Stop()
	case <-timer.C:
	}
}
