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

package internal

import (
	"context"
	"testing"
	"time"

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"google.golang.org/api/option"
	pubsubpb "google.golang.org/genproto/googleapis/pubsub/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCancelStream(t *testing.T) {
	ctx := context.Background()
	srv := pstest.NewServer()
	defer srv.Close()

	var copts []option.ClientOption
	var dialOpts []grpc.DialOption
	conn, err := grpc.Dial(srv.Addr, append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))...)
	assert.NoError(t, err)
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

	client, err := pubsub.NewSubscriberClient(ctx, copts...)
	assert.NoError(t, err)

	handler, err := NewHandler(context.Background(), zaptest.NewLogger(t), client, "client-id", "projects/my-project/subscriptions/otlp",
		func(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
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
