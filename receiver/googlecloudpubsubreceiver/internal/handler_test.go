// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"io"
	"math"
	"sync"
	"testing"
	"time"

	pubsub "cloud.google.com/go/pubsub/v2/apiv1"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/v2/pstest"
	"github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

func createHandler(ctx context.Context, t *testing.T) (cleanupFn func(), srv *pstest.Server, handler *StreamHandler) {
	srv = pstest.NewServer()

	var copts []option.ClientOption
	var dialOpts []grpc.DialOption
	conn, err := grpc.NewClient(srv.Addr, append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))...)
	assert.NoError(t, err)

	cleanupFn = func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, conn.Close())
	}

	copts = append(copts, option.WithGRPCConn(conn))
	_, err = srv.GServer.CreateTopic(ctx, &pubsubpb.Topic{
		Name: "projects/my-project/topics/otlp",
	})
	assert.NoError(t, err)
	_, err = srv.GServer.CreateSubscription(ctx, &pubsubpb.Subscription{
		Topic:              "projects/my-project/topics/otlp",
		Name:               "projects/my-project/subscriptions/otlp",
		AckDeadlineSeconds: 10,
	})
	assert.NoError(t, err)

	settings := receivertest.NewNopSettings(metadata.Type)
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(settings.TelemetrySettings)

	client, err := pubsub.NewSubscriptionAdminClient(ctx, copts...)
	assert.NoError(t, err)
	handler, err = NewHandler(ctx, settings, telemetryBuilder, client, "client-id", "projects/my-project/subscriptions/otlp",
		nil, func(context.Context, *pubsubpb.ReceivedMessage) error {
			return nil
		})
	assert.NoError(t, err)
	return cleanupFn, srv, handler
}

func TestCancelStream(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cleanupFn, srv, handler := createHandler(ctx, t)
	defer cleanupFn()

	srv.Publish("projects/my-project/topics/otlp", []byte{}, map[string]string{
		"ce-type":      "org.opentelemetry.otlp.traces.v1",
		"content-type": "application/protobuf",
	})
	handler.RecoverableStream(ctx)
	go func() {
		time.Sleep(100 * time.Millisecond)
		handler.CancelNow()
	}()
	handler.Wait()
}

// fakeStream is a Subscriber_StreamingPullClient that records the requests
// sent on it. Recv blocks until CloseSend, mimicking a server that half-closes
// the response stream when the client closes the send side.
type fakeStream struct {
	pubsubpb.Subscriber_StreamingPullClient
	requests  chan *pubsubpb.StreamingPullRequest
	closed    chan struct{}
	closeOnce sync.Once
}

func (f *fakeStream) Send(req *pubsubpb.StreamingPullRequest) error {
	f.requests <- req
	return nil
}

func (f *fakeStream) Recv() (*pubsubpb.StreamingPullResponse, error) {
	<-f.closed
	return nil, io.EOF
}

func (f *fakeStream) CloseSend() error {
	f.closeOnce.Do(func() { close(f.closed) })
	return nil
}

// fakeSubscriberClient is a SubscriberClient that hands out a fakeStream and
// records the unary acknowledge requests.
type fakeSubscriberClient struct {
	stream       *fakeStream
	ackRequests  chan *pubsubpb.AcknowledgeRequest
	nackRequests chan *pubsubpb.ModifyAckDeadlineRequest
}

func (*fakeSubscriberClient) Close() error { return nil }

func (c *fakeSubscriberClient) StreamingPull(context.Context, ...gax.CallOption) (pubsubpb.Subscriber_StreamingPullClient, error) {
	return c.stream, nil
}

func (c *fakeSubscriberClient) Acknowledge(_ context.Context, req *pubsubpb.AcknowledgeRequest, _ ...gax.CallOption) error {
	c.ackRequests <- req
	return nil
}

func (c *fakeSubscriberClient) ModifyAckDeadline(_ context.Context, req *pubsubpb.ModifyAckDeadlineRequest, _ ...gax.CallOption) error {
	c.nackRequests <- req
	return nil
}

func createFakeHandler(t *testing.T, config *FlowControlConfig) (*fakeSubscriberClient, *StreamHandler) {
	t.Helper()
	client := &fakeSubscriberClient{
		stream: &fakeStream{
			requests: make(chan *pubsubpb.StreamingPullRequest, 16),
			closed:   make(chan struct{}),
		},
		ackRequests:  make(chan *pubsubpb.AcknowledgeRequest, 16),
		nackRequests: make(chan *pubsubpb.ModifyAckDeadlineRequest, 16),
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	require.NoError(t, err)
	handler, err := NewHandler(t.Context(), settings, telemetryBuilder, client, "client-id",
		"projects/my-project/subscriptions/otlp", config,
		func(context.Context, *pubsubpb.ReceivedMessage) error {
			return nil
		})
	require.NoError(t, err)
	// drain the initial request sent by initStream
	initial := <-client.stream.requests
	assert.Equal(t, "projects/my-project/subscriptions/otlp", initial.Subscription)
	return client, handler
}

func receiveRequest(t *testing.T, requests chan *pubsubpb.StreamingPullRequest) *pubsubpb.StreamingPullRequest {
	t.Helper()
	select {
	case request := <-requests:
		return request
	case <-time.After(5 * time.Second):
		t.Fatal("expected a streaming pull request, got none")
		return nil
	}
}

func TestAcknowledgeMessagesFlushesAcksAndNacks(t *testing.T) {
	client, handler := createFakeHandler(t, nil)

	handler.ack("ack-1")
	handler.nack("nack-1")
	handler.nack("nack-2")
	require.NoError(t, handler.acknowledgeMessages())

	request := receiveRequest(t, client.stream.requests)
	assert.Equal(t, []string{"ack-1"}, request.AckIds)
	assert.Equal(t, []string{"nack-1", "nack-2"}, request.ModifyDeadlineAckIds)
	assert.Equal(t, []int32{0, 0}, request.ModifyDeadlineSeconds)

	// a successful flush clears the pending acknowledgements
	require.NoError(t, handler.acknowledgeMessages())
	assert.Empty(t, client.stream.requests)
}

func TestInitStreamResendsPendingAcksAndNacks(t *testing.T) {
	client, handler := createFakeHandler(t, nil)

	handler.ack("ack-1")
	handler.nack("nack-1")
	require.NoError(t, handler.initStream(t.Context()))

	request := receiveRequest(t, client.stream.requests)
	assert.Equal(t, "projects/my-project/subscriptions/otlp", request.Subscription)
	assert.Equal(t, []string{"ack-1"}, request.AckIds)
	assert.Equal(t, []string{"nack-1"}, request.ModifyDeadlineAckIds)
	assert.Equal(t, []int32{0}, request.ModifyDeadlineSeconds)
}

func TestSizeTriggeredAcknowledgeFlush(t *testing.T) {
	config := NewDefaultFlowControlConfig()
	// the timer must not fire during the test, only the size trigger may flush
	config.TriggerAckBatchDuration = time.Hour
	config.TriggerAckBatchSize = 1000
	client, handler := createFakeHandler(t, config)

	handler.RecoverableStream(t.Context())
	defer handler.CancelNow()

	for i := range 999 {
		handler.ack(fmt.Sprintf("ack-%d", i))
	}
	// below the threshold nothing is flushed
	time.Sleep(100 * time.Millisecond)
	assert.Empty(t, client.stream.requests)

	// the 1000th pending acknowledgement (an ack or a nack) triggers the flush
	handler.nack("nack-999")
	request := receiveRequest(t, client.stream.requests)
	assert.Len(t, request.AckIds, 999)
	assert.Equal(t, []string{"nack-999"}, request.ModifyDeadlineAckIds)
	assert.Equal(t, []int32{0}, request.ModifyDeadlineSeconds)
}

func TestCancelNowFlushesPendingViaUnaryRPC(t *testing.T) {
	config := NewDefaultFlowControlConfig()
	config.TriggerAckBatchDuration = time.Hour
	client, handler := createFakeHandler(t, config)

	handler.RecoverableStream(t.Context())
	handler.ack("ack-1")
	handler.ack("ack-2")
	handler.nack("nack-1")
	handler.CancelNow()

	select {
	case request := <-client.ackRequests:
		assert.Equal(t, "projects/my-project/subscriptions/otlp", request.Subscription)
		assert.Equal(t, []string{"ack-1", "ack-2"}, request.AckIds)
	default:
		t.Fatal("expected pending acks to be flushed over the unary Acknowledge RPC on shutdown")
	}
	select {
	case request := <-client.nackRequests:
		assert.Equal(t, "projects/my-project/subscriptions/otlp", request.Subscription)
		assert.Equal(t, []string{"nack-1"}, request.AckIds)
		assert.Equal(t, int32(0), request.AckDeadlineSeconds)
	default:
		t.Fatal("expected pending nacks to be flushed over the unary ModifyAckDeadline RPC on shutdown")
	}
}

func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		retry int
		max   time.Duration
	}{
		{
			retry: 0,
			max:   time.Duration(0),
		},
	}
	for i := 1; i <= 11; i++ {
		maxBackoff := min(time.Duration(250.0*math.Pow(2, float64(i-1)))*time.Millisecond, time.Duration(2)*time.Minute)
		tests = append(tests, struct {
			retry int
			max   time.Duration
		}{
			retry: i,
			max:   maxBackoff,
		})
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("retry-%d", tt.retry), func(t *testing.T) {
			for range 10 {
				backoff := exponentialBackoff(tt.retry)
				minBackoffDueToJitter := time.Duration(0.7*float64(tt.max.Milliseconds())) * time.Millisecond
				assert.Condition(t, func() bool { return backoff <= tt.max },
					"exponentialBackoff %s should not go over max %s", backoff.String(), tt.max.String())
				assert.Condition(t, func() bool { return backoff >= minBackoffDueToJitter },
					"exponentialBackoff %s should not go under min (due to jitter) %s", backoff.String(), minBackoffDueToJitter.String())
			}
		})
	}
}
