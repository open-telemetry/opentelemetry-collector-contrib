// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// connectAndReceive with connect failure
// connectAndReceive with lifecycle validation
//   not started, connecting, connected, terminating, terminated, idle

func TestReceiveMessage(t *testing.T) {
	someError := errors.New("some error")

	validateMetrics := func(receivedMsgVal, droppedMsgVal, fatalUnmarshalling, reportedSpan interface{}) func(t *testing.T, receiver *solaceTracesReceiver) {
		return func(t *testing.T, receiver *solaceTracesReceiver) {
			validateReceiverMetrics(t, receiver, receivedMsgVal, droppedMsgVal, fatalUnmarshalling, reportedSpan)
		}
	}

	cases := []struct {
		name         string
		nextConsumer consumertest.Consumer
		// errors to return from messagingService.receive, unmarshaller.unmarshal, messagingService.ack and messagingService.nack
		receiveMessageErr, unmarshalErr, ackErr, nackErr error
		// whether or not to expect a nack call instead of an ack
		expectNack bool
		// expected error from receiveMessage
		expectedErr error
		// validate constraints after the fact
		validation func(t *testing.T, receiver *solaceTracesReceiver)
	}{
		{ // no errors, expect no error, validate metrics
			name:       "Receive Message Success",
			validation: validateMetrics(1, nil, nil, 1),
		},
		{ // fail at receiveMessage and expect the error
			name:              "Receive Messages Error",
			receiveMessageErr: someError,
			expectedErr:       someError,
			validation:        validateMetrics(nil, nil, nil, nil),
		},
		{ // unmarshal error expecting the error to be swallowed, the message to be acknowledged, stats incremented
			name:         "Unmarshal Error",
			unmarshalErr: errUnknownTopic,
			validation:   validateMetrics(1, 1, 1, nil),
		},
		{ // unmarshal error with wrong version expecting error to be propagated, message to be rejected
			name:         "Unmarshal Version Error",
			unmarshalErr: errUpgradeRequired,
			expectedErr:  errUpgradeRequired,
			expectNack:   true,
			validation:   validateMetrics(1, nil, 1, nil),
		},
		{ // expect forward to error and message to be swallowed with ack, no error returned
			name:         "Forward Permanent Error",
			nextConsumer: consumertest.NewErr(consumererror.NewPermanent(errors.New("a permanent error"))),
			validation:   validateMetrics(1, 1, nil, nil),
		},
		{ // expect forward to error and message to be swallowed with ack which fails returning an error
			name:         "Forward Permanent Error with Ack Error",
			nextConsumer: consumertest.NewErr(consumererror.NewPermanent(errors.New("a permanent error"))),
			ackErr:       someError,
			expectedErr:  someError,
			validation:   validateMetrics(1, 1, nil, nil),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			receiver, messagingService, unmarshaller := newReceiver(t)
			if testCase.nextConsumer != nil {
				receiver.nextConsumer = testCase.nextConsumer
			}

			msg := &inboundMessage{}
			trace := ptrace.NewTraces()

			// populate mock messagingService and unmarshaller functions, expecting them each to be called at most once
			var receiveMessagesCalled, ackCalled, nackCalled, unmarshalCalled bool
			messagingService.receiveMessageFunc = func(ctx context.Context) (*inboundMessage, error) {
				assert.False(t, receiveMessagesCalled)
				receiveMessagesCalled = true
				if testCase.receiveMessageErr != nil {
					return nil, testCase.receiveMessageErr
				}
				return msg, nil
			}
			messagingService.ackFunc = func(ctx context.Context, msg *inboundMessage) error {
				assert.False(t, ackCalled)
				ackCalled = true
				if testCase.ackErr != nil {
					return testCase.ackErr
				}
				return nil
			}
			messagingService.nackFunc = func(ctx context.Context, msg *inboundMessage) error {
				assert.False(t, nackCalled)
				nackCalled = true
				if testCase.nackErr != nil {
					return testCase.nackErr
				}
				return nil
			}
			unmarshaller.unmarshalFunc = func(msg *inboundMessage) (ptrace.Traces, error) {
				assert.False(t, unmarshalCalled)
				unmarshalCalled = true
				if testCase.unmarshalErr != nil {
					return ptrace.Traces{}, testCase.unmarshalErr
				}
				return trace, nil
			}

			err := receiver.receiveMessage(context.Background(), messagingService)
			if testCase.expectedErr != nil {
				assert.Equal(t, testCase.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
			assert.True(t, receiveMessagesCalled)
			if testCase.receiveMessageErr == nil {
				assert.True(t, unmarshalCalled)
				assert.Equal(t, testCase.expectNack, nackCalled)
				assert.Equal(t, !testCase.expectNack, ackCalled)
			}
			if testCase.validation != nil {
				testCase.validation(t, receiver)
			}
		})
	}
}

// receiveMessages ctx done return
func TestReceiveMessagesTerminateWithCtxDone(t *testing.T) {
	receiver, messagingService, unmarshaller := newReceiver(t)
	receiveMessagesCalled := false
	ctx, cancel := context.WithCancel(context.Background())
	msg := &inboundMessage{}
	trace := ptrace.NewTraces()
	messagingService.receiveMessageFunc = func(ctx context.Context) (*inboundMessage, error) {
		assert.False(t, receiveMessagesCalled)
		receiveMessagesCalled = true
		return msg, nil
	}
	ackCalled := false
	messagingService.ackFunc = func(ctx context.Context, msg *inboundMessage) error {
		assert.False(t, ackCalled)
		ackCalled = true
		cancel()
		return nil
	}
	unmarshalCalled := false
	unmarshaller.unmarshalFunc = func(msg *inboundMessage) (ptrace.Traces, error) {
		assert.False(t, unmarshalCalled)
		unmarshalCalled = true
		return trace, nil
	}
	err := receiver.receiveMessages(ctx, messagingService)
	assert.NoError(t, err)
	assert.True(t, receiveMessagesCalled)
	assert.True(t, unmarshalCalled)
	assert.True(t, ackCalled)
	validateReceiverMetrics(t, receiver, 1, nil, nil, 1)
}

func TestReceiverLifecycle(t *testing.T) {
	receiver, messagingService, _ := newReceiver(t)
	dialCalled := make(chan struct{})
	messagingService.dialFunc = func(context.Context) error {
		validateMetric(t, receiver.metrics.views.receiverStatus, receiverStateConnecting)
		validateMetric(t, receiver.metrics.views.flowControlStatus, flowControlStateClear)
		close(dialCalled)
		return nil
	}
	closeCalled := make(chan struct{})
	messagingService.closeFunc = func(ctx context.Context) {
		validateMetric(t, receiver.metrics.views.receiverStatus, receiverStateTerminating)
		close(closeCalled)
	}
	receiveMessagesCalled := make(chan struct{})
	messagingService.receiveMessageFunc = func(ctx context.Context) (*inboundMessage, error) {
		validateMetric(t, receiver.metrics.views.receiverStatus, receiverStateConnected)
		close(receiveMessagesCalled)
		<-ctx.Done()
		return nil, errors.New("some error")
	}
	// start the receiver
	err := receiver.Start(context.Background(), nil)
	assert.NoError(t, err)
	assertChannelClosed(t, dialCalled)
	assertChannelClosed(t, receiveMessagesCalled)
	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
	assertChannelClosed(t, closeCalled)
	validateMetric(t, receiver.metrics.views.receiverStatus, receiverStateTerminated)
	// we error on receive message, so we should not report any metrics
	validateReceiverMetrics(t, receiver, nil, nil, nil, nil)
}

func TestReceiverDialFailureContinue(t *testing.T) {
	receiver, msgService, _ := newReceiver(t)
	dialErr := errors.New("Some dial error")
	const expectedAttempts = 3 // the number of attempts to perform prior to resolving
	dialCalled := 0
	factoryCalled := 0
	closeCalled := 0
	dialDone := make(chan struct{})
	factoryDone := make(chan struct{})
	closeDone := make(chan struct{})
	receiver.factory = func() messagingService {
		factoryCalled++
		if factoryCalled == expectedAttempts {
			close(factoryDone)
		}
		return msgService
	}
	msgService.dialFunc = func(context.Context) error {
		dialCalled++
		if dialCalled == expectedAttempts {
			close(dialDone)
		}
		return dialErr
	}
	msgService.closeFunc = func(ctx context.Context) {
		closeCalled++
		// asset we never left connecting state prior to closing closeDone
		validateMetric(t, receiver.metrics.views.receiverStatus, receiverStateConnecting)
		if closeCalled == expectedAttempts {
			close(closeDone)
			<-ctx.Done() // wait for ctx.Done
		}
	}
	// start the receiver
	err := receiver.Start(context.Background(), nil)
	assert.NoError(t, err)

	// expect factory to be called twice
	assertChannelClosed(t, factoryDone)
	// expect dial to be called twice
	assertChannelClosed(t, dialDone)
	// expect close to be called twice
	assertChannelClosed(t, closeDone)
	// assert failed reconnections
	validateMetric(t, receiver.metrics.views.failedReconnections, expectedAttempts)

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
	validateMetric(t, receiver.metrics.views.receiverStatus, receiverStateTerminated)
	// we error on dial, should never get to receive messages
	validateReceiverMetrics(t, receiver, nil, nil, nil, nil)
}

func TestReceiverUnmarshalVersionFailureExpectingDisable(t *testing.T) {
	receiver, msgService, unmarshaller := newReceiver(t)
	dialDone := make(chan struct{})
	nackCalled := make(chan struct{})
	closeDone := make(chan struct{})
	unmarshaller.unmarshalFunc = func(msg *inboundMessage) (ptrace.Traces, error) {
		return ptrace.Traces{}, errUpgradeRequired
	}
	msgService.dialFunc = func(context.Context) error {
		// after we receive an unmarshalling version error, we should not call dial again
		msgService.dialFunc = func(context.Context) error {
			t.Error("did not expect dial to be called again")
			return nil
		}
		close(dialDone)
		return nil
	}
	msgService.receiveMessageFunc = func(ctx context.Context) (*inboundMessage, error) {
		// we only expect a single receiveMessage call when unmarshal returns unknown version
		msgService.receiveMessageFunc = func(ctx context.Context) (*inboundMessage, error) {
			t.Error("did not expect receiveMessage to be called again")
			return nil, nil
		}
		return nil, nil
	}
	msgService.nackFunc = func(ctx context.Context, msg *inboundMessage) error {
		close(nackCalled)
		return nil
	}
	msgService.closeFunc = func(ctx context.Context) {
		close(closeDone)
	}
	// start the receiver
	err := receiver.Start(context.Background(), nil)
	assert.NoError(t, err)

	// expect dial to be called twice
	assertChannelClosed(t, dialDone)
	// expect nack to be called
	assertChannelClosed(t, nackCalled)
	// expect close to be called twice
	assertChannelClosed(t, closeDone)
	// we receive 1 message, encounter a fatal unmarshalling error and we nack the message so it is not actually dropped
	validateReceiverMetrics(t, receiver, 1, nil, 1, nil)
	// assert idle state
	validateMetric(t, receiver.metrics.views.receiverStatus, receiverStateIdle)

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
	validateMetric(t, receiver.metrics.views.receiverStatus, receiverStateTerminated)
}

func TestReceiverFlowControlDelayedRetry(t *testing.T) {
	someError := consumererror.NewPermanent(fmt.Errorf("some error"))
	testCases := []struct {
		name         string
		nextConsumer consumer.Traces
		validation   func(*testing.T, *opencensusMetrics)
	}{
		{
			name:         "Without error",
			nextConsumer: consumertest.NewNop(),
		},
		{
			name:         "With error",
			nextConsumer: consumertest.NewErr(someError),
			validation: func(t *testing.T, metrics *opencensusMetrics) {
				validateMetric(t, metrics.views.droppedSpanMessages, 1)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			receiver, messagingService, unmarshaller := newReceiver(t)
			delay := 50 * time.Millisecond
			// Increase delay on windows due to tick granularity
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17197
			if runtime.GOOS == "windows" {
				delay = 500 * time.Millisecond
			}
			receiver.config.Flow.DelayedRetry.Delay = delay
			var err error
			// we want to return an error at first, then set the next consumer to a noop consumer
			receiver.nextConsumer, err = consumer.NewTraces(func(ctx context.Context, ld ptrace.Traces) error {
				receiver.nextConsumer = tc.nextConsumer
				return fmt.Errorf("Some temporary error")
			})
			require.NoError(t, err)

			// populate mock messagingService and unmarshaller functions, expecting them each to be called at most once
			var ackCalled bool
			messagingService.ackFunc = func(ctx context.Context, msg *inboundMessage) error {
				assert.False(t, ackCalled)
				ackCalled = true
				return nil
			}
			messagingService.receiveMessageFunc = func(ctx context.Context) (*inboundMessage, error) {
				return &inboundMessage{}, nil
			}
			unmarshaller.unmarshalFunc = func(msg *inboundMessage) (ptrace.Traces, error) {
				return ptrace.NewTraces(), nil
			}

			receiveMessageComplete := make(chan error, 1)
			go func() {
				receiveMessageComplete <- receiver.receiveMessage(context.Background(), messagingService)
			}()
			select {
			case <-time.After(delay / 2):
				// success
			case <-receiveMessageComplete:
				require.Fail(t, "Did not expect receiveMessage to return before delay interval")
			}
			// Check that we are currently flow controlled
			validateMetric(t, receiver.metrics.views.flowControlStatus, flowControlStateControlled)
			// since we set the next consumer to a noop, this should succeed
			select {
			case <-time.After(delay):
				require.Fail(t, "receiveMessage did not return after delay interval")
			case err := <-receiveMessageComplete:
				assert.NoError(t, err)
			}
			assert.True(t, ackCalled)
			if tc.validation != nil {
				tc.validation(t, receiver.metrics)
			}
			validateMetric(t, receiver.metrics.views.flowControlRecentRetries, 1)
			validateMetric(t, receiver.metrics.views.flowControlStatus, flowControlStateClear)
			validateMetric(t, receiver.metrics.views.flowControlTotal, 1)
			validateMetric(t, receiver.metrics.views.flowControlSingleSuccess, 1)
		})
	}
}

func TestReceiverFlowControlDelayedRetryInterrupt(t *testing.T) {
	receiver, messagingService, unmarshaller := newReceiver(t)
	// we won't wait 10 seconds since we will interrupt well before
	receiver.config.Flow.DelayedRetry.Delay = 10 * time.Second
	var err error
	// we want to return an error at first, then set the next consumer to a noop consumer
	receiver.nextConsumer, err = consumer.NewTraces(func(ctx context.Context, ld ptrace.Traces) error {
		// if we are called again, fatal
		receiver.nextConsumer, err = consumer.NewTraces(func(ctx context.Context, ld ptrace.Traces) error {
			require.Fail(t, "Did not expect next consumer to be called again")
			return nil
		})
		require.NoError(t, err)
		return fmt.Errorf("Some temporary error")
	})
	require.NoError(t, err)

	// populate mock messagingService and unmarshaller functions, expecting them each to be called at most once
	messagingService.receiveMessageFunc = func(ctx context.Context) (*inboundMessage, error) {
		return &inboundMessage{}, nil
	}
	unmarshaller.unmarshalFunc = func(msg *inboundMessage) (ptrace.Traces, error) {
		return ptrace.NewTraces(), nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	receiveMessageComplete := make(chan error, 1)
	go func() {
		receiveMessageComplete <- receiver.receiveMessage(ctx, messagingService)
	}()
	select {
	case <-time.After(2 * time.Millisecond):
		// success
	case <-receiveMessageComplete:
		require.Fail(t, "Did not expect receiveMessage to return before delay interval")
	}
	cancel()
	// since we set the next consumer to a noop, this should succeed
	select {
	case <-time.After(2 * time.Millisecond):
		require.Fail(t, "receiveMessage did not return after some time")
	case err := <-receiveMessageComplete:
		assert.ErrorContains(t, err, "delayed retry interrupted by shutdown request")
	}
}

func TestReceiverFlowControlDelayedRetryMultipleRetries(t *testing.T) {
	receiver, messagingService, unmarshaller := newReceiver(t)
	// we won't wait 10 seconds since we will interrupt well before
	retryInterval := 50 * time.Millisecond
	// Increase delay on windows due to tick granularity
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/19409
	if runtime.GOOS == "windows" {
		retryInterval = 500 * time.Millisecond
	}
	var retryCount int64 = 5
	receiver.config.Flow.DelayedRetry.Delay = retryInterval
	var err error
	var currentRetries int64
	// we want to return an error at first, then set the next consumer to a noop consumer
	receiver.nextConsumer, err = consumer.NewTraces(func(ctx context.Context, ld ptrace.Traces) error {
		if currentRetries > 0 {
			validateMetric(t, receiver.metrics.views.flowControlRecentRetries, currentRetries)
		}
		currentRetries++
		if currentRetries == retryCount {
			receiver.nextConsumer, err = consumer.NewTraces(func(ctx context.Context, ld ptrace.Traces) error {
				return nil
			})
		}
		require.NoError(t, err)
		return fmt.Errorf("Some temporary error")
	})
	require.NoError(t, err)

	// populate mock messagingService and unmarshaller functions, expecting them each to be called at most once
	var ackCalled bool
	messagingService.ackFunc = func(ctx context.Context, msg *inboundMessage) error {
		assert.False(t, ackCalled)
		ackCalled = true
		return nil
	}
	messagingService.receiveMessageFunc = func(ctx context.Context) (*inboundMessage, error) {
		return &inboundMessage{}, nil
	}
	unmarshaller.unmarshalFunc = func(msg *inboundMessage) (ptrace.Traces, error) {
		return ptrace.NewTraces(), nil
	}

	receiveMessageComplete := make(chan error, 1)
	go func() {
		receiveMessageComplete <- receiver.receiveMessage(context.Background(), messagingService)
	}()
	select {
	case <-time.After(retryInterval * time.Duration(retryCount) / 2):
		// success
	case <-receiveMessageComplete:
		require.Fail(t, "Did not expect receiveMessage to return before delay interval")
	}
	validateMetric(t, receiver.metrics.views.flowControlStatus, flowControlStateControlled)
	// since we set the next consumer to a noop, this should succeed
	select {
	case <-time.After(2 * retryInterval * time.Duration(retryCount)):
		require.Fail(t, "receiveMessage did not return after some time")
	case err := <-receiveMessageComplete:
		assert.NoError(t, err)
	}
	assert.True(t, ackCalled)
	validateMetric(t, receiver.metrics.views.flowControlRecentRetries, retryCount)
	validateMetric(t, receiver.metrics.views.flowControlStatus, flowControlStateClear)
	validateMetric(t, receiver.metrics.views.flowControlTotal, 1)
	validateMetric(t, receiver.metrics.views.flowControlSingleSuccess, nil)
}

func newReceiver(t *testing.T) (*solaceTracesReceiver, *mockMessagingService, *mockUnmarshaller) {
	unmarshaller := &mockUnmarshaller{}
	service := &mockMessagingService{}
	messagingServiceFactory := func() messagingService {
		return service
	}
	metrics := newTestMetrics(t)
	receiver := &solaceTracesReceiver{
		settings: receivertest.NewNopCreateSettings(),
		config: &Config{
			Flow: FlowControl{
				DelayedRetry: &FlowControlDelayedRetry{
					Delay: 10 * time.Millisecond,
				},
			},
		},
		nextConsumer:      consumertest.NewNop(),
		metrics:           metrics,
		unmarshaller:      unmarshaller,
		factory:           messagingServiceFactory,
		shutdownWaitGroup: &sync.WaitGroup{},
		retryTimeout:      1 * time.Millisecond,
		terminating:       &atomic.Bool{},
	}
	return receiver, service, unmarshaller
}

func validateReceiverMetrics(t *testing.T, receiver *solaceTracesReceiver, receivedMsgVal, droppedMsgVal, fatalUnmarshalling, reportedSpan interface{}) {
	validateMetric(t, receiver.metrics.views.receivedSpanMessages, receivedMsgVal)
	validateMetric(t, receiver.metrics.views.droppedSpanMessages, droppedMsgVal)
	validateMetric(t, receiver.metrics.views.fatalUnmarshallingErrors, fatalUnmarshalling)
	validateMetric(t, receiver.metrics.views.reportedSpans, reportedSpan)
}

type mockMessagingService struct {
	dialFunc           func(ctx context.Context) error
	closeFunc          func(ctx context.Context)
	receiveMessageFunc func(ctx context.Context) (*inboundMessage, error)
	ackFunc            func(ctx context.Context, msg *inboundMessage) error
	nackFunc           func(ctx context.Context, msg *inboundMessage) error
}

func (m *mockMessagingService) dial(ctx context.Context) error {
	if m.dialFunc != nil {
		return m.dialFunc(ctx)
	}
	panic("did not expect dial to be called")
}

func (m *mockMessagingService) close(ctx context.Context) {
	if m.closeFunc != nil {
		m.closeFunc(ctx)
		return
	}
	panic("did not expect close to be called")
}

func (m *mockMessagingService) receiveMessage(ctx context.Context) (*inboundMessage, error) {
	if m.receiveMessageFunc != nil {
		return m.receiveMessageFunc(ctx)
	}
	panic("did not expect receiveMessage to be called")
}

func (m *mockMessagingService) accept(ctx context.Context, msg *inboundMessage) error {
	if m.ackFunc != nil {
		return m.ackFunc(ctx, msg)
	}
	panic("did not expect ack to be called")
}

func (m *mockMessagingService) failed(ctx context.Context, msg *inboundMessage) error {
	if m.nackFunc != nil {
		return m.nackFunc(ctx, msg)
	}
	panic("did not expect nack to be called")
}

type mockUnmarshaller struct {
	unmarshalFunc func(msg *inboundMessage) (ptrace.Traces, error)
}

func (m *mockUnmarshaller) unmarshal(message *inboundMessage) (ptrace.Traces, error) {
	if m.unmarshalFunc != nil {
		return m.unmarshalFunc(message)
	}
	panic("did not expect unmarshal to be called")
}
