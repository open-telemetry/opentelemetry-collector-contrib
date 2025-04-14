// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadata"
)

type receiverState uint8

const (
	receiverStateStarting receiverState = iota
	receiverStateConnecting
	receiverStateConnected
	receiverStateIdle
	receiverStateTerminating
	receiverStateTerminated
)

type flowControlState uint8

const (
	flowControlStateClear flowControlState = iota
	flowControlStateControlled
)

const (
	brokerComponentNameAttr = "receiver_name"
)

// solaceTracesReceiver uses azure AMQP to consume and handle telemetry data from SOlace. Implements receiver.Traces
type solaceTracesReceiver struct {
	// config is the receiver.Config instance used to build the receiver
	config *Config

	nextConsumer     consumer.Traces
	settings         receiver.Settings
	telemetryBuilder *metadata.TelemetryBuilder
	unmarshaller     tracesUnmarshaller
	// cancel is the function that will cancel the context associated with the main worker loop
	cancel            context.CancelFunc
	shutdownWaitGroup *sync.WaitGroup
	// newFactory is the constructor to use to build new messagingServiceFactory instances
	factory messagingServiceFactory
	// terminating is used to indicate that the receiver is terminating
	terminating *atomic.Bool
	// retryTimeout is the timeout between connection attempts
	retryTimeout time.Duration
	// Other Attributes including the ID of the receiver Solace broker's component name
	metricAttrs attribute.Set
}

// newTracesReceiver creates a new solaceTraceReceiver as a receiver.Traces
func newTracesReceiver(config *Config, set receiver.Settings, nextConsumer consumer.Traces) (receiver.Traces, error) {
	factory, err := newAMQPMessagingServiceFactory(config, set.Logger)
	if err != nil {
		set.Logger.Warn("Error validating messaging service configuration", zap.Error(err))
		return nil, err
	}

	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		set.Logger.Warn("Error registering metrics", zap.Error(err))
		return nil, err
	}

	// solaceBrokerAttrs - including the component name of the connected Solace broker
	receiverName := set.ID.Name()
	if receiverName != "" {
		receiverName = "solace/" + receiverName
	} else {
		receiverName = "solace"
	}
	solaceBrokerAttrs := attribute.NewSet(
		attribute.String(brokerComponentNameAttr, receiverName),
	)

	unmarshaller := newTracesUnmarshaller(set.Logger, telemetryBuilder, solaceBrokerAttrs)

	return &solaceTracesReceiver{
		config:            config,
		nextConsumer:      nextConsumer,
		settings:          set,
		telemetryBuilder:  telemetryBuilder,
		unmarshaller:      unmarshaller,
		shutdownWaitGroup: &sync.WaitGroup{},
		factory:           factory,
		retryTimeout:      1 * time.Second,
		terminating:       &atomic.Bool{},
		metricAttrs:       solaceBrokerAttrs,
	}, nil
}

// Start implements component.Receiver::Start
func (s *solaceTracesReceiver) Start(ctx context.Context, _ component.Host) error {
	// set the component name for the connected Solace broker
	s.telemetryBuilder.SolacereceiverReceiverStatus.Record(ctx, int64(receiverStateStarting), metric.WithAttributeSet(s.metricAttrs))
	s.telemetryBuilder.SolacereceiverReceiverFlowControlStatus.Record(ctx, int64(flowControlStateClear), metric.WithAttributeSet(s.metricAttrs))
	var cancelableContext context.Context
	cancelableContext, s.cancel = context.WithCancel(context.Background())

	s.settings.Logger.Info("Starting receiver")
	// start the reconnection loop with a cancellable context and a factory to build new messaging services
	go s.connectAndReceive(cancelableContext)

	s.settings.Logger.Info("Receiver successfully started")
	return nil
}

// Shutdown implements component.Receiver::Shutdown
func (s *solaceTracesReceiver) Shutdown(_ context.Context) error {
	if s.cancel == nil {
		return nil
	}
	// set the component name for the connected Solace broker
	s.terminating.Store(true)
	s.telemetryBuilder.SolacereceiverReceiverStatus.Record(context.Background(), int64(receiverStateTerminating), metric.WithAttributeSet(s.metricAttrs))
	s.settings.Logger.Info("Shutdown waiting for all components to complete")
	s.cancel() // cancels the context passed to the reconnection loop
	s.shutdownWaitGroup.Wait()
	s.settings.Logger.Info("Receiver shutdown successfully")
	s.telemetryBuilder.SolacereceiverReceiverStatus.Record(context.Background(), int64(receiverStateTerminated), metric.WithAttributeSet(s.metricAttrs))
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
	s.telemetryBuilder.SolacereceiverReceiverStatus.Record(context.Background(), int64(receiverStateConnecting), metric.WithAttributeSet(s.metricAttrs))

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

			if err := service.dial(ctx); err != nil {
				s.settings.Logger.Debug("Encountered error while connecting messaging service", zap.Error(err))
				s.telemetryBuilder.SolacereceiverFailedReconnections.Add(ctx, 1, metric.WithAttributeSet(s.metricAttrs))
				return
			}
			// dial was successful, record the connected state
			s.recordConnectionState(receiverStateConnected)

			if err := s.receiveMessages(ctx, service); err != nil {
				s.settings.Logger.Debug("Encountered error while receiving messages", zap.Error(err))
				if errors.Is(err, errUpgradeRequired) {
					s.telemetryBuilder.SolacereceiverNeedUpgrade.Record(ctx, 1, metric.WithAttributeSet(s.metricAttrs))
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
		s.telemetryBuilder.SolacereceiverReceiverStatus.Record(context.Background(), int64(state), metric.WithAttributeSet(s.metricAttrs))
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
	disposition := service.accept
	defer func() { // on return of receiveMessage, we want to either ack or nack the message
		if disposition != nil {
			if actionErr := disposition(ctx, msg); err == nil && actionErr != nil {
				err = actionErr
			}
		}
	}()
	// message received successfully
	s.telemetryBuilder.SolacereceiverReceivedSpanMessages.Add(ctx, 1, metric.WithAttributeSet(s.metricAttrs))
	// unmarshal the message. unmarshalling errors are not fatal unless the version is unknown
	traces, unmarshalErr := s.unmarshaller.unmarshal(msg)
	if unmarshalErr != nil {
		s.settings.Logger.Error("Encountered error while unmarshalling message", zap.Error(unmarshalErr))
		s.telemetryBuilder.SolacereceiverFatalUnmarshallingErrors.Add(ctx, 1, metric.WithAttributeSet(s.metricAttrs))
		if errors.Is(unmarshalErr, errUpgradeRequired) {
			disposition = service.failed // if we don't know the version, reject the trace message since we will disable the receiver
			return unmarshalErr
		}
		s.telemetryBuilder.SolacereceiverDroppedSpanMessages.Add(ctx, 1, metric.WithAttributeSet(s.metricAttrs)) // if the error is some other unmarshalling error, we will ack the message and drop the content
		return nil                                                                                               // don't propagate error, but don't continue forwarding traces
	}

	var flowControlCount int64
	var spanCount int
flowControlLoop:
	for {
		// forward to next consumer. Forwarding errors are not fatal so are not propagated to the caller.
		// Temporary consumer errors will lead to redelivered messages, permanent will be accepted
		if (traces != ptrace.Traces{}) {
			spanCount = traces.SpanCount() // get the span count into a variable before we call consumeTraces
		}

		forwardErr := s.nextConsumer.ConsumeTraces(ctx, traces)
		if forwardErr == nil {
			// no forward error
			s.telemetryBuilder.SolacereceiverReportedSpans.Add(ctx, int64(spanCount), metric.WithAttributeSet(s.metricAttrs))
			break flowControlLoop
		}
		if consumererror.IsPermanent(forwardErr) { // error is permanent, we want to accept the message and increment the number of dropped messages
			s.settings.Logger.Warn("Encountered permanent error while forwarding traces to next receiver, will swallow trace", zap.Error(forwardErr))
			s.telemetryBuilder.SolacereceiverDroppedSpanMessages.Add(ctx, 1, metric.WithAttributeSet(s.metricAttrs))
			break flowControlLoop
		}
		s.settings.Logger.Info("Encountered temporary error while forwarding traces to next receiver, will allow redelivery", zap.Error(forwardErr))
		// handle flow control metrics
		if flowControlCount == 0 {
			s.telemetryBuilder.SolacereceiverReceiverFlowControlStatus.Record(ctx, int64(flowControlStateControlled), metric.WithAttributeSet(s.metricAttrs))
		}
		flowControlCount++
		s.telemetryBuilder.SolacereceiverReceiverFlowControlRecentRetries.Record(ctx, flowControlCount, metric.WithAttributeSet(s.metricAttrs))
		// Backpressure scenario. For now, we are only delayed retry, eventually we may need to handle this
		delayTimer := time.NewTimer(s.config.Flow.DelayedRetry.Delay)
		select {
		case <-delayTimer.C:
			continue flowControlLoop
		case <-ctx.Done():
			s.settings.Logger.Info("Context was cancelled while attempting redelivery, exiting")
			disposition = nil // do not make any network requests, we are shutting down
			return errors.New("delayed retry interrupted by shutdown request")
		}
	}
	// Make sure to clear the stats no matter what, unless we were interrupted in which case we should preserve the last state
	if flowControlCount != 0 {
		s.telemetryBuilder.SolacereceiverReceiverFlowControlStatus.Record(ctx, int64(flowControlStateClear), metric.WithAttributeSet(s.metricAttrs))
		s.telemetryBuilder.SolacereceiverReceiverFlowControlTotal.Add(ctx, 1, metric.WithAttributeSet(s.metricAttrs))
		if flowControlCount == 1 {
			s.telemetryBuilder.SolacereceiverReceiverFlowControlWithSingleSuccessfulRetry.Add(ctx, 1, metric.WithAttributeSet(s.metricAttrs))
		}
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
