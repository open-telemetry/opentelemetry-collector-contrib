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

package gcloudpubsubexporter

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

const name = "gcloudpubsub"

// pubsubExporter is a wrapper struct of OT cloud trace exporter
type pubsubExporter struct {
	instanceName string
	logger       *zap.Logger

	tracesTopicName  string
	metricsTopicName string
	logsTopicName    string

	tracesTopic  *pubsub.Topic
	metricsTopic *pubsub.Topic
	logsTopic    *pubsub.Topic

	//
	userAgent string
	config    *Config
	//
	client *pubsub.Client
}

func (*pubsubExporter) Name() string {
	return name
}

func (ex *pubsubExporter) Start(ctx context.Context, _ component.Host) error {
	if ex.client == nil {
		copts, _ := ex.generateClientOptions()
		client, _ := pubsub.NewClient(context.Background(), ex.config.ProjectID, copts...)
		ex.client = client
	}

	if ex.tracesTopic == nil && ex.tracesTopicName != "" {
		ex.tracesTopic = ex.client.TopicInProject(ex.tracesTopicName, ex.config.ProjectID)
		if ex.config.ValidateExistence {
			tctx, cancel := context.WithTimeout(ctx, ex.config.Timeout)
			defer cancel()
			exist, err := ex.tracesTopic.Exists(tctx)
			if err != nil {
				return err
			}
			if !exist {
				return fmt.Errorf("trace subscription %s doesn't exist", ex.tracesTopic)
			}
		}
	}
	if ex.metricsTopic == nil && ex.metricsTopicName != "" {
		ex.metricsTopic = ex.client.TopicInProject(ex.metricsTopicName, ex.config.ProjectID)
		if ex.config.ValidateExistence {
			tctx, cancel := context.WithTimeout(ctx, ex.config.Timeout)
			defer cancel()
			exist, err := ex.metricsTopic.Exists(tctx)
			if err != nil {
				return err
			}
			if !exist {
				return fmt.Errorf("metric subscription %s doesn't exist", ex.tracesTopic)
			}
		}
	}
	if ex.logsTopic == nil && ex.logsTopicName != "" {
		ex.logsTopic = ex.client.TopicInProject(ex.logsTopicName, ex.config.ProjectID)
		if ex.config.ValidateExistence {
			tctx, cancel := context.WithTimeout(ctx, ex.config.Timeout)
			defer cancel()
			exist, err := ex.logsTopic.Exists(tctx)
			if err != nil {
				return err
			}
			if !exist {
				return fmt.Errorf("log subscription %s doesn't exist", ex.tracesTopic)
			}
		}
	}
	return nil
}

func (ex *pubsubExporter) Shutdown(context.Context) error {
	if ex.tracesTopic != nil {
		ex.tracesTopic.Stop()
		ex.tracesTopic = nil
	}
	if ex.metricsTopic != nil {
		ex.metricsTopic.Stop()
		ex.metricsTopic = nil
	}
	if ex.logsTopic != nil {
		ex.logsTopic.Stop()
		ex.logsTopic = nil
	}
	if ex.client != nil {
		ex.client.Close()
		ex.client = nil
	}
	return nil
}

func (ex *pubsubExporter) generateClientOptions() ([]option.ClientOption, error) {
	var copts []option.ClientOption
	if ex.userAgent != "" {
		copts = append(copts, option.WithUserAgent(ex.userAgent))
	}
	if ex.config.Endpoint != "" {
		if ex.config.UseInsecure {
			var dialOpts []grpc.DialOption
			if ex.userAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(ex.userAgent))
			}
			conn, _ := grpc.Dial(ex.config.Endpoint, append(dialOpts, grpc.WithInsecure())...)
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(ex.config.Endpoint))
		}
	}
	return copts, nil
}

func (ex *pubsubExporter) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	bytes, _ := td.ToOtlpProtoBytes()
	message := &pubsub.Message{Data: bytes}
	_ = ex.tracesTopic.Publish(ctx, message)
	return nil
}

func (ex *pubsubExporter) ConsumeMetrics(ctx context.Context, td pdata.Metrics) error {
	bytes, _ := td.ToOtlpProtoBytes()
	message := &pubsub.Message{Data: bytes}
	_ = ex.metricsTopic.Publish(ctx, message)
	return nil
}

func (ex *pubsubExporter) ConsumeLogs(ctx context.Context, td pdata.Logs) error {
	bytes, _ := td.ToOtlpProtoBytes()
	message := &pubsub.Message{Data: bytes}
	_ = ex.logsTopic.Publish(ctx, message)
	return nil
}
