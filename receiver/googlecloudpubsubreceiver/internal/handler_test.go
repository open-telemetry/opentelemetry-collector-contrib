// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"testing"
	"time"

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

func TestCancelStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := pstest.NewServer()
	defer srv.Close()

	var copts []option.ClientOption
	var dialOpts []grpc.DialOption
	conn, err := grpc.NewClient(srv.Addr, append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))...)
	assert.NoError(t, err)
	defer func() { assert.NoError(t, conn.Close()) }()
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

	settings := receivertest.NewNopSettings()
	telemetryBuilder, _ := metadata.NewTelemetryBuilder(settings.TelemetrySettings)

	client, err := pubsub.NewSubscriberClient(ctx, copts...)
	assert.NoError(t, err)

	handler, err := NewHandler(ctx, settings, telemetryBuilder, client, "client-id", "projects/my-project/subscriptions/otlp",
		func(context.Context, *pubsubpb.ReceivedMessage) error {
			return nil
		})
	handler.ackBatchWait = 10 * time.Millisecond
	assert.NoError(t, err)
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
