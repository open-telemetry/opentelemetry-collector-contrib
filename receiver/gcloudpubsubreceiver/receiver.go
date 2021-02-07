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
	pubsub "cloud.google.com/go/pubsub/apiv1"
	"context"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	pubsubpb "google.golang.org/genproto/googleapis/pubsub/v1"
	"google.golang.org/grpc"
	"sync"
)

// https://cloud.google.com/pubsub/docs/reference/rpc/google.pubsub.v1#streamingpullrequest
type pubsubReceiver struct {
	instanceName    string
	logger          *zap.Logger
	tracesConsumer  consumer.TracesConsumer
	metricsConsumer consumer.MetricsConsumer
	logsConsumer    consumer.LogsConsumer
	userAgent       string
	config          *Config
	client          *pubsub.SubscriberClient

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (receiver *pubsubReceiver) generateClientOptions() ([]option.ClientOption, error) {
	var copts []option.ClientOption
	if receiver.userAgent != "" {
		copts = append(copts, option.WithUserAgent(receiver.userAgent))
	}
	if receiver.config.Endpoint != "" {
		if receiver.config.UseInsecure {
			var dialOpts []grpc.DialOption
			if receiver.userAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(receiver.userAgent))
			}
			conn, _ := grpc.Dial(receiver.config.Endpoint, append(dialOpts, grpc.WithInsecure())...)
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(receiver.config.Endpoint))
		}
	}
	return copts, nil
}

func (receiver *pubsubReceiver) Start(ctx context.Context, _ component.Host) error {
	ctx, receiver.cancel = context.WithCancel(ctx)

	if receiver.client == nil {
		copts, _ := receiver.generateClientOptions()
		client, err := pubsub.NewSubscriberClient(ctx, copts...)
		if err != nil {
			return fmt.Errorf("failed creating the gRPC client to Pubsub: %w", err)
		}

		receiver.client = client
	}

	if receiver.tracesConsumer != nil {
		err := receiver.createTracesReceiverHandler(ctx)
		if err != nil {
			receiver.cancel()
			return fmt.Errorf("failed to create TracesReceiverHandler: %w", err)
		}
	}
	if receiver.metricsConsumer != nil {
		err := receiver.createMetricsReceiverHandler(ctx)
		if err != nil {
			receiver.cancel()
			return fmt.Errorf("failed to create MetricsReceiverHandler: %w", err)
		}
	}
	if receiver.logsConsumer != nil {
		err := receiver.createLogsReceiverHandler(ctx)
		if err != nil {
			receiver.cancel()
			return fmt.Errorf("failed to create LogReceiverHandler: %w", err)
		}
	}
	return nil
}

func (receiver *pubsubReceiver) Shutdown(_ context.Context) error {
	receiver.cancel()
	receiver.logger.Info("Stopping Google Pubsub receiver")
	receiver.wg.Wait()
	receiver.logger.Info("Stopped Google Pubsub receiver")
	return nil
}

func (receiver *pubsubReceiver) createTracesReceiverHandler(ctx context.Context) error {
	handler := streamHandler{
		receiver: receiver,
		pushMessage: func(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
			otlpData := pdata.NewTraces()
			err := otlpData.FromOtlpProtoBytes(message.Message.Data)
			if err != nil {
				return err
			}
			return receiver.tracesConsumer.ConsumeTraces(ctx, otlpData)
		},
		acks: make([]string, 0),
	}
	subscription := fmt.Sprintf("projects/%s/subscriptions/%s",
		handler.receiver.config.ProjectID,
		handler.receiver.config.TracesSubscription)
	err := handler.initStream(ctx, subscription)
	if err != nil {
		return err
	}
	receiver.wg.Add(1)
	go handler.recoverableStream(ctx, subscription)
	return nil
}

func (receiver *pubsubReceiver) createMetricsReceiverHandler(ctx context.Context) error {
	handler := streamHandler{
		receiver: receiver,
		pushMessage: func(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
			otlpData := pdata.NewMetrics()
			err := otlpData.FromOtlpProtoBytes(message.Message.Data)
			if err != nil {
				return err
			}
			return receiver.metricsConsumer.ConsumeMetrics(ctx, otlpData)
		},
		acks: make([]string, 0),
	}
	subscription := fmt.Sprintf("projects/%s/subscriptions/%s",
		handler.receiver.config.ProjectID,
		handler.receiver.config.MetricsSubscription)
	err := handler.initStream(ctx, subscription)
	if err != nil {
		return err
	}
	receiver.wg.Add(1)
	go handler.recoverableStream(ctx, subscription)
	return nil
}

func (receiver *pubsubReceiver) createLogsReceiverHandler(ctx context.Context) error {
	handler := streamHandler{
		receiver: receiver,
		pushMessage: func(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
			otlpData := pdata.NewLogs()
			err := otlpData.FromOtlpProtoBytes(message.Message.Data)
			if err != nil {
				return err
			}
			return receiver.logsConsumer.ConsumeLogs(ctx, otlpData)
		},
		acks: make([]string, 0),
	}
	subscription := fmt.Sprintf("projects/%s/subscriptions/%s",
		handler.receiver.config.ProjectID,
		handler.receiver.config.LogsSubscription)
	err := handler.initStream(ctx, subscription)
	if err != nil {
		return err
	}
	receiver.wg.Add(1)
	go handler.recoverableStream(ctx, subscription)
	return nil
}
