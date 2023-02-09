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

package googlecloudpubsubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"

import (
	"context"
	"errors"
	"fmt"

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"
)

// https://cloud.google.com/pubsub/docs/reference/rpc/google.pubsub.v1#streamingpullrequest
type pubsubPullReceiver struct {
	*pubsubReceiver
	userAgent string
	client    *pubsub.SubscriberClient
	handler   *internal.StreamHandler
}

func (receiver *pubsubPullReceiver) generateClientOptions() (copts []option.ClientOption) {
	if receiver.userAgent != "" {
		copts = append(copts, option.WithUserAgent(receiver.userAgent))
	}
	if receiver.config.Endpoint != "" {
		if receiver.config.Insecure {
			var dialOpts []grpc.DialOption
			if receiver.userAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(receiver.userAgent))
			}
			conn, _ := grpc.Dial(receiver.config.Endpoint, append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))...)
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(receiver.config.Endpoint))
		}
	}
	return copts
}

func (receiver *pubsubPullReceiver) Start(ctx context.Context, _ component.Host) error {
	var startErr error
	receiver.startOnce.Do(func() {
		copts := receiver.generateClientOptions()
		client, err := pubsub.NewSubscriberClient(ctx, copts...)
		if err != nil {
			startErr = fmt.Errorf("failed creating the gRPC client to Pubsub: %w", err)
			return
		}
		receiver.client = client

		err = receiver.createReceiverHandler(ctx)
		if err != nil {
			startErr = fmt.Errorf("failed to create ReceiverHandler: %w", err)
			return
		}
	})
	receiver.tracesUnmarshaler = &ptrace.ProtoUnmarshaler{}
	receiver.metricsUnmarshaler = &pmetric.ProtoUnmarshaler{}
	receiver.logsUnmarshaler = &plog.ProtoUnmarshaler{}
	return startErr
}

func (receiver *pubsubPullReceiver) Shutdown(_ context.Context) error {
	receiver.logger.Info("Stopping Google Pubsub receiver")
	receiver.handler.CancelNow()
	receiver.logger.Info("Stopped Google Pubsub receiver")
	return nil
}

func (receiver *pubsubPullReceiver) handleTextPull(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
	timestamp := message.GetMessage().PublishTime
	return receiver.handleLogStrings(ctx, message.Message.Data, timestamp.AsTime())
}

func (receiver *pubsubPullReceiver) createReceiverHandler(ctx context.Context) error {
	var err error
	receiver.handler, err = internal.NewHandler(
		ctx,
		receiver.logger,
		receiver.client,
		receiver.config.ClientID,
		receiver.config.Subscription,
		func(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
			payload := message.Message.Data
			encoding, compression := detectEncoding(receiver.config, message.Message.Attributes)

			switch encoding {
			case otlpProtoTrace:
				if receiver.tracesConsumer != nil {
					return receiver.handleTrace(ctx, payload, compression)
				}
			case otlpProtoMetric:
				if receiver.metricsConsumer != nil {
					return receiver.handleMetric(ctx, payload, compression)
				}
			case otlpProtoLog:
				if receiver.logsConsumer != nil {
					return receiver.handleLog(ctx, payload, compression)
				}
			case rawTextLog:
				return receiver.handleTextPull(ctx, message)
			}
			return errors.New("unknown encoding")
		})
	if err != nil {
		return err
	}
	receiver.handler.RecoverableStream(ctx)
	return nil
}
