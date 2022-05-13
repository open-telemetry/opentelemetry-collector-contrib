// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulsarreceiver

import (
	"context"
	"errors"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

var errUnrecognizedEncoding = errors.New("unrecognized encoding")

type PulsarTracesConsumer struct {
	id              config.ComponentID
	tracesConsumer  consumer.Traces
	topic           string
	client          pulsar.Client
	cancel          context.CancelFunc
	consumer        pulsar.Consumer
	ready           chan bool
	unmarshaler     TracesUnmarshaler
	settings        component.ReceiverCreateSettings
	consumerOptions pulsar.ConsumerOptions
}

func newTracesReceiver(config Config, set component.ReceiverCreateSettings, unmarshalers map[string]TracesUnmarshaler, nextConsumer consumer.Traces) (*PulsarTracesConsumer, error) {
	unmarshaler := unmarshalers[config.Encoding]
	if nil == unmarshaler {
		return nil, errUnrecognizedEncoding
	}

	options, err := config.clientOptions()
	if err != nil {
		return nil, err
	}
	client, err := pulsar.NewClient(options)
	if err != nil {
		return nil, err
	}

	consumerOptions, err := config.consumerOptions()
	if err != nil {
		return nil, err
	}

	return &PulsarTracesConsumer{
		id:              config.ID(),
		tracesConsumer:  nextConsumer,
		topic:           config.Topic,
		unmarshaler:     unmarshaler,
		settings:        set,
		client:          client,
		ready:           make(chan bool),
		consumerOptions: consumerOptions,
	}, nil
}

func (c *PulsarTracesConsumer) Start(context.Context, component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		err := consumerTracesLoop(ctx, c)
		if err != nil {
			c.settings.Logger.Error("Consume tracesConsumer failed", zap.Error(err))
		}
	}()

	<-c.ready
	return nil
}

func consumerTracesLoop(ctx context.Context, c *PulsarTracesConsumer) error {
	unmarshaler := c.unmarshaler
	traceConsumer := c.tracesConsumer
	close(c.ready)

	_consumer, err := c.client.Subscribe(c.consumerOptions)
	if nil != err {
		return err
	}

	c.consumer = _consumer

	for true {
		message, err := _consumer.Receive(ctx)
		if err != nil {
			if value, ok := err.(*pulsar.Error); ok && value.Result() == pulsar.AlreadyClosedError {
				return err
			} else {
				time.Sleep(time.Second)
				continue
			}
		}

		traces, err := unmarshaler.Unmarshal(message.Payload())
		if err != nil {
			c.settings.Logger.Error("unmarshaler message failed", zap.Error(err))
		}

		if err := traceConsumer.ConsumeTraces(context.Background(), traces); err != nil {
			c.settings.Logger.Error("consume traces failed", zap.Error(err))
		}
		c.consumer.Ack(message)
	}

	return nil
}

func (c *PulsarTracesConsumer) Shutdown(context.Context) error {
	c.cancel()
	c.consumer.Close()
	c.client.Close()
	return nil
}

type PulsarMetricsConsumer struct {
	id              config.ComponentID
	metricsConsumer consumer.Metrics
	unmarshaler     MetricsUnmarshaler
	topic           string
	client          pulsar.Client
	consumer        pulsar.Consumer
	ready           chan bool
	cancel          context.CancelFunc
	settings        component.ReceiverCreateSettings
	consumerOptions pulsar.ConsumerOptions
}

func newMetricsReceiver(config Config, set component.ReceiverCreateSettings, unmarshalers map[string]MetricsUnmarshaler, nextConsumer consumer.Metrics) (*PulsarMetricsConsumer, error) {
	unmarshaler := unmarshalers[config.Encoding]
	if nil == unmarshaler {
		return nil, errUnrecognizedEncoding
	}

	options, err := config.clientOptions()
	if err != nil {
		return nil, err
	}
	client, err := pulsar.NewClient(options)
	if err != nil {
		return nil, err
	}

	consumerOptions, err := config.consumerOptions()
	if err != nil {
		return nil, err
	}

	return &PulsarMetricsConsumer{
		id:              config.ID(),
		metricsConsumer: nextConsumer,
		topic:           config.Topic,
		unmarshaler:     unmarshaler,
		settings:        set,
		client:          client,
		ready:           make(chan bool),
		consumerOptions: consumerOptions,
	}, nil
}

func (c *PulsarMetricsConsumer) Start(context.Context, component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		err := consumeMetricsLoop(ctx, c)
		if err != nil {
			c.settings.Logger.Error("consume metrics loop occurs an error", zap.Error(err))
		}
	}()

	<-c.ready
	return nil
}

func consumeMetricsLoop(ctx context.Context, c *PulsarMetricsConsumer) error {
	unmarshaler := c.unmarshaler
	close(c.ready)

	_consumer, err := c.client.Subscribe(c.consumerOptions)
	if nil != err {
		return err
	}

	c.consumer = _consumer

	for true {
		message, err := _consumer.Receive(ctx)
		if err != nil {
			if value, ok := err.(*pulsar.Error); ok && value.Result() == pulsar.AlreadyClosedError {
				return err
			} else {
				time.Sleep(time.Second)
				continue
			}
		}

		metrics, err := unmarshaler.Unmarshal(message.Payload())
		if err != nil {
			c.settings.Logger.Error("unmarshaler message failed", zap.Error(err))
		}

		if err := c.metricsConsumer.ConsumeMetrics(context.Background(), metrics); err != nil {
			c.settings.Logger.Error("consume traces failed", zap.Error(err))
		}

		c.consumer.Ack(message)
	}

	return nil
}

func (c *PulsarMetricsConsumer) Shutdown(context.Context) error {
	c.cancel()
	c.consumer.Close()
	c.client.Close()
	return nil
}

type PulsarLogsConsumer struct {
	id              config.ComponentID
	logsConsumer    consumer.Logs
	unmarshaler     LogsUnmarshaler
	topic           string
	client          pulsar.Client
	consumer        pulsar.Consumer
	ready           chan bool
	cancel          context.CancelFunc
	settings        component.ReceiverCreateSettings
	consumerOptions pulsar.ConsumerOptions
}

func newLogsReceiver(config Config, set component.ReceiverCreateSettings, unmarshalers map[string]LogsUnmarshaler, nextConsumer consumer.Logs) (*PulsarLogsConsumer, error) {
	unmarshaler := unmarshalers[config.Encoding]
	if nil == unmarshaler {
		return nil, errUnrecognizedEncoding
	}

	options, err := config.clientOptions()
	if err != nil {
		return nil, err
	}
	client, err := pulsar.NewClient(options)
	if err != nil {
		return nil, err
	}

	consumerOptions, err := config.consumerOptions()
	if err != nil {
		return nil, err
	}

	return &PulsarLogsConsumer{
		id:              config.ID(),
		logsConsumer:    nextConsumer,
		topic:           config.Topic,
		cancel:          nil,
		unmarshaler:     unmarshaler,
		settings:        set,
		client:          client,
		ready:           make(chan bool),
		consumerOptions: consumerOptions,
	}, nil
}

func (c *PulsarLogsConsumer) Start(context.Context, component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		err := consumeLogsLoop(ctx, c)
		if err != nil {
			c.settings.Logger.Error("consume logs loop occurs an error")
		}
	}()

	<-c.ready
	return nil
}

func consumeLogsLoop(ctx context.Context, c *PulsarLogsConsumer) error {
	unmarshaler := c.unmarshaler
	close(c.ready)

	_consumer, err := c.client.Subscribe(c.consumerOptions)

	if nil != err {
		return err
	}

	c.consumer = _consumer

	for true {
		message, err := _consumer.Receive(ctx)
		if err != nil {
			if value, ok := err.(*pulsar.Error); ok && value.Result() == pulsar.AlreadyClosedError {
				return err
			} else {
				time.Sleep(time.Second)
				continue
			}
		}

		logs, err := unmarshaler.Unmarshal(message.Payload())
		if err != nil {
			c.settings.Logger.Error("unmarshaler message failed", zap.Error(err))
		}

		if err := c.logsConsumer.ConsumeLogs(context.Background(), logs); err != nil {
			c.settings.Logger.Error("consume traces failed", zap.Error(err))
		}

		c.consumer.Ack(message)
	}

	return nil
}

func (c *PulsarLogsConsumer) Shutdown(context.Context) error {
	c.cancel()
	c.consumer.Close()
	c.client.Close()
	return nil
}
