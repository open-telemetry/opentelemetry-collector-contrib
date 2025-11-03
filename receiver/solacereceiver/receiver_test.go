// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadatatest"
)

// connectAndReceive with connect failure
// connectAndReceive with lifecycle validation
//   not started, connecting, connected, terminating, terminated, idle

func TestReceiveMessage(t *testing.T) {
	someError := errors.New("some error")
	validateMetrics := func(receivedMsgVal, droppedMsgVal, fatalUnmarshalling, reportedSpan int64) func(t *testing.T, tt *componenttest.Telemetry) {
		return func(t *testing.T, tt *componenttest.Telemetry) {
			if reportedSpan > 0 {
				metadatatest.AssertEqualSolacereceiverReportedSpans(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: reportedSpan,
					},
				}, metricdatatest.IgnoreTimestamp())
			}
			if receivedMsgVal > 0 {
				metadatatest.AssertEqualSolacereceiverReceivedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: receivedMsgVal,
					},
				}, metricdatatest.IgnoreTimestamp())
			}
			if droppedMsgVal > 0 {
				metadatatest.AssertEqualSolacereceiverDroppedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: droppedMsgVal,
					},
				}, metricdatatest.IgnoreTimestamp())
			}
			if fatalUnmarshalling > 0 {
				metadatatest.AssertEqualSolacereceiverFatalUnmarshallingErrors(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: fatalUnmarshalling,
					},
				}, metricdatatest.IgnoreTimestamp())
			}
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
		validation func(t *testing.T, tt *componenttest.Telemetry)
		// traces provided by the trace function
		traces ptrace.Traces
	}{
		{ // no errors, expect no error, validate metrics
			name:       "Receive Message Success",
			validation: validateMetrics(1, 0, 0, 1),
			traces:     newTestTracesWithSpans(1),
		},
		{ // no errors, expect no error, validate metrics
			name:       "Receive Message Multiple Traces Success",
			validation: validateMetrics(1, 0, 0, 3),
			traces:     newTestTracesWithSpans(3),
		},
		{ // fail at receiveMessage and expect the error
			name:              "Receive Messages Error",
			receiveMessageErr: someError,
			expectedErr:       someError,
			validation:        validateMetrics(0, 0, 0, 0),
		},
		{ // unmarshal error expecting the error to be swallowed, the message to be acknowledged, stats incremented
			name:         "Unmarshal Error",
			unmarshalErr: errUnknownTopic,
			validation:   validateMetrics(1, 1, 1, 0),
		},
		{ // unmarshal error with wrong version expecting error to be propagated, message to be rejected
			name:         "Unmarshal Version Error",
			unmarshalErr: errUpgradeRequired,
			expectedErr:  errUpgradeRequired,
			expectNack:   true,
			validation:   validateMetrics(1, 0, 1, 0),
		},
		{ // expect forward to error and message to be swallowed with ack, no error returned
			name:         "Forward Permanent Error",
			nextConsumer: consumertest.NewErr(consumererror.NewPermanent(errors.New("a permanent error"))),
			validation:   validateMetrics(1, 1, 0, 0),
		},
		{ // expect forward to error and message to be swallowed with ack which fails returning an error
			name:         "Forward Permanent Error with Ack Error",
			nextConsumer: consumertest.NewErr(consumererror.NewPermanent(errors.New("a permanent error"))),
			ackErr:       someError,
			expectedErr:  someError,
			validation:   validateMetrics(1, 1, 0, 0),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			receiver, messagingService, unmarshaller, tt := newReceiver(t)
			if testCase.nextConsumer != nil {
				receiver.nextConsumer = testCase.nextConsumer
			}

			msg := &inboundMessage{}

			// populate mock messagingService and unmarshaller functions, expecting them each to be called at most once
			var receiveMessagesCalled, ackCalled, nackCalled, unmarshalCalled bool
			messagingService.receiveMessageFunc = func(context.Context) (*inboundMessage, error) {
				assert.False(t, receiveMessagesCalled)
				receiveMessagesCalled = true
				if testCase.receiveMessageErr != nil {
					return nil, testCase.receiveMessageErr
				}
				return msg, nil
			}
			messagingService.ackFunc = func(context.Context, *inboundMessage) error {
				assert.False(t, ackCalled)
				ackCalled = true
				if testCase.ackErr != nil {
					return testCase.ackErr
				}
				return nil
			}
			messagingService.nackFunc = func(context.Context, *inboundMessage) error {
				assert.False(t, nackCalled)
				nackCalled = true
				if testCase.nackErr != nil {
					return testCase.nackErr
				}
				return nil
			}
			unmarshaller.unmarshalFunc = func(*inboundMessage) (ptrace.Traces, error) {
				assert.False(t, unmarshalCalled)
				unmarshalCalled = true
				if testCase.unmarshalErr != nil {
					return ptrace.Traces{}, testCase.unmarshalErr
				}
				return testCase.traces, nil
			}

			err := receiver.receiveMessage(t.Context(), messagingService)
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
				testCase.validation(t, tt)
			}
		})
	}
}

// receiveMessages ctx done return
func TestReceiveMessagesTerminateWithCtxDone(t *testing.T) {
	receiver, messagingService, unmarshaller, tt := newReceiver(t)
	receiveMessagesCalled := false
	ctx, cancel := context.WithCancel(t.Context())
	msg := &inboundMessage{}
	trace := newTestTracesWithSpans(1)
	messagingService.receiveMessageFunc = func(context.Context) (*inboundMessage, error) {
		assert.False(t, receiveMessagesCalled)
		receiveMessagesCalled = true
		return msg, nil
	}
	ackCalled := false
	messagingService.ackFunc = func(context.Context, *inboundMessage) error {
		assert.False(t, ackCalled)
		ackCalled = true
		cancel()
		return nil
	}
	unmarshalCalled := false
	unmarshaller.unmarshalFunc = func(*inboundMessage) (ptrace.Traces, error) {
		assert.False(t, unmarshalCalled)
		unmarshalCalled = true
		return trace, nil
	}
	err := receiver.receiveMessages(ctx, messagingService)
	assert.NoError(t, err)
	assert.True(t, receiveMessagesCalled)
	assert.True(t, unmarshalCalled)
	assert.True(t, ackCalled)
	metadatatest.AssertEqualSolacereceiverReceivedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: 1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverReportedSpans(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: 1,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestReceiverLifecycle(t *testing.T) {
	receiver, messagingService, _, tt := newReceiver(t)
	dialCalled := make(chan struct{})
	messagingService.dialFunc = func(context.Context) error {
		metadatatest.AssertEqualSolacereceiverReceiverStatus(t, tt, []metricdata.DataPoint[int64]{
			{
				Value: int64(receiverStateConnecting),
			},
		}, metricdatatest.IgnoreTimestamp())
		metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
			{
				Value: int64(flowControlStateClear),
			},
		}, metricdatatest.IgnoreTimestamp())
		close(dialCalled)
		return nil
	}
	closeCalled := make(chan struct{})
	messagingService.closeFunc = func(context.Context) {
		metadatatest.AssertEqualSolacereceiverReceiverStatus(t, tt, []metricdata.DataPoint[int64]{
			{
				Value: int64(receiverStateTerminating),
			},
		}, metricdatatest.IgnoreTimestamp())
		metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
			{
				Value: int64(flowControlStateClear),
			},
		}, metricdatatest.IgnoreTimestamp())
		close(closeCalled)
	}
	receiveMessagesCalled := make(chan struct{})
	messagingService.receiveMessageFunc = func(ctx context.Context) (*inboundMessage, error) {
		metadatatest.AssertEqualSolacereceiverReceiverStatus(t, tt, []metricdata.DataPoint[int64]{
			{
				Value: int64(receiverStateConnected),
			},
		}, metricdatatest.IgnoreTimestamp())
		metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
			{
				Value: int64(flowControlStateClear),
			},
		}, metricdatatest.IgnoreTimestamp())
		close(receiveMessagesCalled)
		<-ctx.Done()
		return nil, errors.New("some error")
	}
	// start the receiver
	err := receiver.Start(t.Context(), nil)
	assert.NoError(t, err)
	assertChannelClosed(t, dialCalled)
	assertChannelClosed(t, receiveMessagesCalled)
	err = receiver.Shutdown(t.Context())
	assert.NoError(t, err)
	assertChannelClosed(t, closeCalled)
	// we error on receive message, so we should not report any additional metrics
	metadatatest.AssertEqualSolacereceiverReceiverStatus(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: int64(receiverStateTerminated),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: int64(flowControlStateClear),
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestReceiverDialFailureContinue(t *testing.T) {
	receiver, msgService, _, tt := newReceiver(t)
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
		// assert we never left connecting state prior to closing closeDone
		metadatatest.AssertEqualSolacereceiverReceiverStatus(t, tt, []metricdata.DataPoint[int64]{
			{
				Value: int64(receiverStateConnecting),
			},
		}, metricdatatest.IgnoreTimestamp())
		metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
			{
				Value: int64(flowControlStateClear),
			},
		}, metricdatatest.IgnoreTimestamp())
		metadatatest.AssertEqualSolacereceiverFailedReconnections(t, tt, []metricdata.DataPoint[int64]{
			{
				Value: int64(closeCalled),
			},
		}, metricdatatest.IgnoreTimestamp())
		if closeCalled == expectedAttempts {
			close(closeDone)
			<-ctx.Done() // wait for ctx.Done
		}
	}
	// start the receiver
	err := receiver.Start(t.Context(), nil)
	assert.NoError(t, err)

	// expect factory to be called twice
	assertChannelClosed(t, factoryDone)
	// expect dial to be called twice
	assertChannelClosed(t, dialDone)
	// expect close to be called twice
	assertChannelClosed(t, closeDone)
	// assert failed reconnections

	err = receiver.Shutdown(t.Context())
	assert.NoError(t, err)
	// we error on dial, should never get to receive messages
	metadatatest.AssertEqualSolacereceiverReceiverStatus(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: int64(receiverStateTerminated),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: int64(flowControlStateClear),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverFailedReconnections(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: 3,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestReceiverUnmarshalVersionFailureExpectingDisable(t *testing.T) {
	receiver, msgService, unmarshaller, tt := newReceiver(t)
	dialDone := make(chan struct{})
	nackCalled := make(chan struct{})
	closeDone := make(chan struct{})
	unmarshaller.unmarshalFunc = func(*inboundMessage) (ptrace.Traces, error) {
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
	msgService.receiveMessageFunc = func(context.Context) (*inboundMessage, error) {
		// we only expect a single receiveMessage call when unmarshal returns unknown version
		msgService.receiveMessageFunc = func(context.Context) (*inboundMessage, error) {
			t.Error("did not expect receiveMessage to be called again")
			return nil, nil
		}
		return nil, nil
	}
	msgService.nackFunc = func(context.Context, *inboundMessage) error {
		close(nackCalled)
		return nil
	}
	msgService.closeFunc = func(context.Context) {
		close(closeDone)
	}
	// start the receiver
	err := receiver.Start(t.Context(), nil)
	assert.NoError(t, err)

	// expect dial to be called twice
	assertChannelClosed(t, dialDone)
	// expect nack to be called
	assertChannelClosed(t, nackCalled)
	// expect close to be called twice
	assertChannelClosed(t, closeDone)
	// we receive 1 message, encounter a fatal unmarshalling error and we nack the message so it is not actually dropped
	// assert idle state
	metadatatest.AssertEqualSolacereceiverReceivedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: 1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverFatalUnmarshallingErrors(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: 1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverReceiverStatus(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: int64(receiverStateIdle),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: int64(flowControlStateClear),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverNeedUpgrade(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: 1,
		},
	}, metricdatatest.IgnoreTimestamp())
	err = receiver.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestReceiverFlowControlDelayedRetry(t *testing.T) {
	someError := consumererror.NewPermanent(errors.New("some error"))
	testCases := []struct {
		name         string
		nextConsumer consumer.Traces
		validation   func(*testing.T, *componenttest.Telemetry)
	}{
		{
			name:         "Without error",
			nextConsumer: consumertest.NewNop(),
		},
		{
			name:         "With error",
			nextConsumer: consumertest.NewErr(someError),
			validation: func(t *testing.T, tt *componenttest.Telemetry) {
				metadatatest.AssertEqualSolacereceiverReceivedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 1,
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: int64(flowControlStateClear),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverReceiverFlowControlRecentRetries(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 1,
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverReceiverFlowControlTotal(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 1,
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverDroppedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 1,
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverReceiverFlowControlWithSingleSuccessfulRetry(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 1,
					},
				}, metricdatatest.IgnoreTimestamp())
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			receiver, messagingService, unmarshaller, tt := newReceiver(t)
			delay := 50 * time.Millisecond
			// Increase delay on windows due to tick granularity
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17197
			if runtime.GOOS == "windows" {
				delay = 500 * time.Millisecond
			}
			receiver.config.Flow.DelayedRetry.Get().Delay = delay
			var err error
			// we want to return an error at first, then set the next consumer to a noop consumer
			receiver.nextConsumer, err = consumer.NewTraces(func(context.Context, ptrace.Traces) error {
				receiver.nextConsumer = tc.nextConsumer
				return errors.New("Some temporary error")
			})
			require.NoError(t, err)

			// populate mock messagingService and unmarshaller functions, expecting them each to be called at most once
			var ackCalled bool
			messagingService.ackFunc = func(context.Context, *inboundMessage) error {
				assert.False(t, ackCalled)
				ackCalled = true
				return nil
			}
			messagingService.receiveMessageFunc = func(context.Context) (*inboundMessage, error) {
				return &inboundMessage{}, nil
			}
			unmarshaller.unmarshalFunc = func(*inboundMessage) (ptrace.Traces, error) {
				return ptrace.NewTraces(), nil
			}

			receiveMessageComplete := make(chan error, 1)
			go func() {
				receiveMessageComplete <- receiver.receiveMessage(t.Context(), messagingService)
			}()
			select {
			case <-time.After(delay / 2):
				// success
			case <-receiveMessageComplete:
				require.Fail(t, "Did not expect receiveMessage to return before delay interval")
			}
			// Check that we are currently flow controlled
			metadatatest.AssertEqualSolacereceiverReceivedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
				{
					Value: 1,
				},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
				{
					Value: int64(flowControlStateControlled),
				},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualSolacereceiverReceiverFlowControlRecentRetries(t, tt, []metricdata.DataPoint[int64]{
				{
					Value: 1,
				},
			}, metricdatatest.IgnoreTimestamp())
			// since we set the next consumer to a noop, this should succeed
			select {
			case <-time.After(delay):
				require.Fail(t, "receiveMessage did not return after delay interval")
			case err := <-receiveMessageComplete:
				assert.NoError(t, err)
			}
			assert.True(t, ackCalled)
			if tc.validation != nil {
				tc.validation(t, tt)
			} else {
				metadatatest.AssertEqualSolacereceiverReceivedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 1,
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: int64(flowControlStateClear),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverReceiverFlowControlRecentRetries(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 1,
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverReceiverFlowControlTotal(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 1,
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverReportedSpans(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 0,
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualSolacereceiverReceiverFlowControlWithSingleSuccessfulRetry(t, tt, []metricdata.DataPoint[int64]{
					{
						Value: 1,
					},
				}, metricdatatest.IgnoreTimestamp())
			}
		})
	}
}

func TestReceiverFlowControlDelayedRetryInterrupt(t *testing.T) {
	receiver, messagingService, unmarshaller, _ := newReceiver(t)
	// we won't wait 10 seconds since we will interrupt well before
	receiver.config.Flow.DelayedRetry.Get().Delay = 10 * time.Second
	var err error
	// we want to return an error at first, then set the next consumer to a noop consumer
	receiver.nextConsumer, err = consumer.NewTraces(func(context.Context, ptrace.Traces) error {
		// if we are called again, fatal
		receiver.nextConsumer, err = consumer.NewTraces(func(context.Context, ptrace.Traces) error {
			require.Fail(t, "Did not expect next consumer to be called again")
			return nil
		})
		require.NoError(t, err)
		return errors.New("Some temporary error")
	})
	require.NoError(t, err)

	// populate mock messagingService and unmarshaller functions, expecting them each to be called at most once
	messagingService.receiveMessageFunc = func(context.Context) (*inboundMessage, error) {
		return &inboundMessage{}, nil
	}
	unmarshaller.unmarshalFunc = func(*inboundMessage) (ptrace.Traces, error) {
		return ptrace.NewTraces(), nil
	}

	ctx, cancel := context.WithCancel(t.Context())
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
	receiver, messagingService, unmarshaller, tt := newReceiver(t)
	// we won't wait 10 seconds since we will interrupt well before
	retryInterval := 50 * time.Millisecond
	// Increase delay on windows due to tick granularity
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/19409
	if runtime.GOOS == "windows" {
		retryInterval = 500 * time.Millisecond
	}
	var retryCount int64 = 5
	receiver.config.Flow.DelayedRetry.Get().Delay = retryInterval
	var err error
	var currentRetries int64
	// we want to return an error at first, then set the next consumer to a noop consumer
	receiver.nextConsumer, err = consumer.NewTraces(func(context.Context, ptrace.Traces) error {
		if currentRetries > 0 {
			metadatatest.AssertEqualSolacereceiverReceivedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
				{
					Value: 1,
				},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
				{
					Value: int64(flowControlStateControlled),
				},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualSolacereceiverReceiverFlowControlRecentRetries(t, tt, []metricdata.DataPoint[int64]{
				{
					Value: currentRetries,
				},
			}, metricdatatest.IgnoreTimestamp())
		}
		currentRetries++
		if currentRetries == retryCount {
			receiver.nextConsumer, err = consumer.NewTraces(func(context.Context, ptrace.Traces) error {
				return nil
			})
		}
		require.NoError(t, err)
		return errors.New("Some temporary error")
	})
	require.NoError(t, err)

	// populate mock messagingService and unmarshaller functions, expecting them each to be called at most once
	var ackCalled bool
	messagingService.ackFunc = func(context.Context, *inboundMessage) error {
		assert.False(t, ackCalled)
		ackCalled = true
		return nil
	}
	messagingService.receiveMessageFunc = func(context.Context) (*inboundMessage, error) {
		return &inboundMessage{}, nil
	}
	unmarshaller.unmarshalFunc = func(*inboundMessage) (ptrace.Traces, error) {
		return ptrace.NewTraces(), nil
	}

	receiveMessageComplete := make(chan error, 1)
	go func() {
		receiveMessageComplete <- receiver.receiveMessage(t.Context(), messagingService)
	}()
	select {
	case <-time.After(retryInterval * time.Duration(retryCount) / 2):
		// success
	case <-receiveMessageComplete:
		require.Fail(t, "Did not expect receiveMessage to return before delay interval")
	}
	// since we set the next consumer to a noop, this should succeed
	select {
	case <-time.After(2 * retryInterval * time.Duration(retryCount)):
		require.Fail(t, "receiveMessage did not return after some time")
	case err := <-receiveMessageComplete:
		assert.NoError(t, err)
	}
	assert.True(t, ackCalled)
	metadatatest.AssertEqualSolacereceiverReceivedSpanMessages(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: 1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverReceiverFlowControlStatus(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: int64(flowControlStateClear),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverReceiverFlowControlRecentRetries(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: retryCount,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverReceiverFlowControlTotal(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: 1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSolacereceiverReportedSpans(t, tt, []metricdata.DataPoint[int64]{
		{
			Value: 0,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func newReceiver(t *testing.T) (*solaceTracesReceiver, *mockMessagingService, *mockUnmarshaller, *componenttest.Telemetry) {
	unmarshaller := &mockUnmarshaller{}
	service := &mockMessagingService{}
	messagingServiceFactory := func() messagingService {
		return service
	}
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)
	receiver := &solaceTracesReceiver{
		settings: receivertest.NewNopSettings(metadata.Type),
		config: &Config{
			Flow: FlowControl{
				DelayedRetry: configoptional.Some(FlowControlDelayedRetry{
					Delay: 10 * time.Millisecond,
				}),
			},
		},
		nextConsumer:      consumertest.NewNop(),
		telemetryBuilder:  telemetryBuilder,
		unmarshaller:      unmarshaller,
		factory:           messagingServiceFactory,
		shutdownWaitGroup: &sync.WaitGroup{},
		retryTimeout:      1 * time.Millisecond,
		terminating:       &atomic.Bool{},
	}
	return receiver, service, unmarshaller, tel
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

func newTestTracesWithSpans(spanCount int) ptrace.Traces {
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for range spanCount {
		spans.Spans().AppendEmpty()
	}
	return traces
}
