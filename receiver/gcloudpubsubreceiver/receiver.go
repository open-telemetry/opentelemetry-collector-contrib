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

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

type pubsubReceiver struct {
	instanceName    string
	logger          *zap.Logger
	tracesConsumer  consumer.TracesConsumer
	metricsConsumer consumer.MetricsConsumer
	logsConsumer    consumer.LogsConsumer
	userAgent       string
	config          *Config
	client          *pubsub.Client
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
	if receiver.client == nil {
		copts, _ := receiver.generateClientOptions()
		client, _ := pubsub.NewClient(ctx, receiver.config.ProjectID, copts...)
		receiver.client = client
	}
	if receiver.tracesConsumer != nil {
		subscription := receiver.client.SubscriptionInProject(receiver.config.TracesSubscription, receiver.config.ProjectID)
		if receiver.config.ValidateExistence {
			tctx, cancel := context.WithTimeout(ctx, receiver.config.Timeout)
			defer cancel()
			exist, err := subscription.Exists(tctx)
			if err != nil {
				return err
			}
			if !exist {
				return fmt.Errorf("trace subscription %s doesn't exist", subscription)
			}
		}
		go subscription.Receive(ctx, createTracesReceiverHandler(receiver.tracesConsumer))
	}
	if receiver.metricsConsumer != nil {
		subscription := receiver.client.SubscriptionInProject(receiver.config.MetricsSubscription, receiver.config.ProjectID)
		if receiver.config.ValidateExistence {
			tctx, cancel := context.WithTimeout(ctx, receiver.config.Timeout)
			defer cancel()
			exist, err := subscription.Exists(tctx)
			if err != nil {
				return err
			}
			if !exist {
				return fmt.Errorf("metric subscription %s doesn't exist", subscription)
			}
		}
		go subscription.Receive(ctx, createMetricReceiverHandler(receiver.metricsConsumer))
	}
	if receiver.logsConsumer != nil {
		subscription := receiver.client.SubscriptionInProject(receiver.config.LogsSubscription, receiver.config.ProjectID)
		if receiver.config.ValidateExistence {
			tctx, cancel := context.WithTimeout(ctx, receiver.config.Timeout)
			defer cancel()
			exist, err := subscription.Exists(tctx)
			if err != nil {
				return err
			}
			if !exist {
				return fmt.Errorf("log subscription %s doesn't exist", subscription)
			}
		}
		go subscription.Receive(ctx, createLogReceiverHandler(receiver.logsConsumer))
	}
	return nil
}

func (receiver *pubsubReceiver) Shutdown(_ context.Context) error {
	if receiver.client != nil {
		err := receiver.client.Close()
		receiver.client = nil
		return err
	}
	return nil
}

func createTracesReceiverHandler(consumer consumer.TracesConsumer) func(context.Context, *pubsub.Message) {
	return func(ctx context.Context, m *pubsub.Message) {
		otlpData := pdata.NewTraces()
		otlpData.FromOtlpProtoBytes(m.Data)
		consumer.ConsumeTraces(ctx, otlpData)
		m.Ack()
	}
}

func createMetricReceiverHandler(consumer consumer.MetricsConsumer) func(context.Context, *pubsub.Message) {
	return func(ctx context.Context, m *pubsub.Message) {
		otlpData := pdata.NewMetrics()
		otlpData.FromOtlpProtoBytes(m.Data)
		consumer.ConsumeMetrics(ctx, otlpData)
		m.Ack()
	}
}

func createLogReceiverHandler(consumer consumer.LogsConsumer) func(context.Context, *pubsub.Message) {
	return func(ctx context.Context, m *pubsub.Message) {
		otlpData := pdata.NewLogs()
		otlpData.FromOtlpProtoBytes(m.Data)
		consumer.ConsumeLogs(ctx, otlpData)
		m.Ack()
	}
}
