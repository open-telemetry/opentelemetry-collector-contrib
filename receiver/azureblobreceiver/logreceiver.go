// Copyright OpenTelemetry Authors
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

package azureblobreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type LogsConsumer interface {
	ConsumeLogsJson(ctx context.Context, json string) error
}

type logRceiver struct {
	blobEventHandler BlobEventHandler
	logger           *zap.Logger
	logsUnmarshaler  pdata.LogsUnmarshaler
}

// func (ex *logExporter) onLogData(context context.Context, logData pdata.Logs) error {
// 	buf, err := ex.logsMarshaler.MarshalLogs(logData)
// 	if err != nil {
// 		return err
// 	}

// 	return ex.blobClient.UploadData(buf, config.LogsDataType)
// }

func (l *logRceiver) Start(ctx context.Context, host component.Host) error {
	l.blobEventHandler.SetLogsConsumer(l)

	l.blobEventHandler.Run(ctx)

	return nil
}

func (l *logRceiver) Shutdown(ctx context.Context) error {
	l.blobEventHandler.Close(ctx)

	return nil
}

func (l *logRceiver) ConsumeLogsJson(ctx context.Context, json string) error {
	l.logger.Info("========logs===========")
	l.logger.Info(json)
	return nil
}

// Returns a new instance of the log receiver
func NewLogsReceiver(config Config, set component.ReceiverCreateSettings, nextConsumer consumer.Logs, blobEventHandler BlobEventHandler) (component.LogsReceiver, error) {
	logRceiver := &logRceiver{
		blobEventHandler: blobEventHandler,
		logger:           set.Logger,
		logsUnmarshaler:  otlp.NewJSONLogsUnmarshaler(),
	}

	return logRceiver, nil
}

// func (f *kafkaReceiverFactory) createLogsReceiver(
// 	_ context.Context,
// 	set component.ReceiverCreateSettings,
// 	cfg config.Receiver,
// 	nextConsumer consumer.Logs,
// ) (component.LogsReceiver, error) {
// 	c := cfg.(*Config)
// 	r, err := newLogsReceiver(*c, set, f.logsUnmarshalers, nextConsumer)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return r, nil
// }

// func newLogsReceiver(config Config, set component.ReceiverCreateSettings, unmarshalers map[string]LogsUnmarshaler, nextConsumer consumer.Logs) (*kafkaLogsConsumer, error) {
// 	unmarshaler := unmarshalers[config.Encoding]
// 	if unmarshaler == nil {
// 		return nil, errUnrecognizedEncoding
// 	}

// 	c := sarama.NewConfig()
// 	c.ClientID = config.ClientID
// 	c.Metadata.Full = config.Metadata.Full
// 	c.Metadata.Retry.Max = config.Metadata.Retry.Max
// 	c.Metadata.Retry.Backoff = config.Metadata.Retry.Backoff
// 	if config.ProtocolVersion != "" {
// 		version, err := sarama.ParseKafkaVersion(config.ProtocolVersion)
// 		if err != nil {
// 			return nil, err
// 		}
// 		c.Version = version
// 	}
// 	if err := kafkaexporter.ConfigureAuthentication(config.Authentication, c); err != nil {
// 		return nil, err
// 	}
// 	client, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, c)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &kafkaLogsConsumer{
// 		id:                config.ID(),
// 		consumerGroup:     client,
// 		topics:            []string{config.Topic},
// 		nextConsumer:      nextConsumer,
// 		unmarshaler:       unmarshaler,
// 		settings:          set,
// 		autocommitEnabled: config.AutoCommit.Enable,
// 		messageMarking:    config.MessageMarking,
// 	}, nil
// }

// func (c *kafkaLogsConsumer) Start(context.Context, component.Host) error {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	c.cancelConsumeLoop = cancel
// 	logsConsumerGroup := &logsConsumerGroupHandler{
// 		id:           c.id,
// 		logger:       c.settings.Logger,
// 		unmarshaler:  c.unmarshaler,
// 		nextConsumer: c.nextConsumer,
// 		ready:        make(chan bool),
// 		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
// 			ReceiverID:             c.id,
// 			Transport:              transport,
// 			ReceiverCreateSettings: c.settings,
// 		}),
// 		autocommitEnabled: c.autocommitEnabled,
// 		messageMarking:    c.messageMarking,
// 	}
// 	go c.consumeLoop(ctx, logsConsumerGroup)
// 	<-logsConsumerGroup.ready
// 	return nil
// }

// func (c *kafkaLogsConsumer) consumeLoop(ctx context.Context, handler sarama.ConsumerGroupHandler) error {
// 	for {
// 		// `Consume` should be called inside an infinite loop, when a
// 		// server-side rebalance happens, the consumer session will need to be
// 		// recreated to get the new claims
// 		if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
// 			c.settings.Logger.Error("Error from consumer", zap.Error(err))
// 		}
// 		// check if context was cancelled, signaling that the consumer should stop
// 		if ctx.Err() != nil {
// 			c.settings.Logger.Info("Consumer stopped", zap.Error(ctx.Err()))
// 			return ctx.Err()
// 		}
// 	}
// }

// func (c *kafkaLogsConsumer) Shutdown(context.Context) error {
// 	c.cancelConsumeLoop()
// 	return c.consumerGroup.Close()
// }
