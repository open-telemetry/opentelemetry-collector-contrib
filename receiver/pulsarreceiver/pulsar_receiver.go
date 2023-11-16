// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

var errIncorrectEncoding = "encoding %s doesn't implement %s"

const alreadyClosedError = "AlreadyClosedError"

type pulsarTracesConsumer struct {
	tracesConsumer  consumer.Traces
	topic           string
	client          pulsar.Client
	cancel          context.CancelFunc
	consumer        pulsar.Consumer
	unmarshaler     encoding.TracesUnmarshalerExtension
	settings        receiver.CreateSettings
	consumerOptions pulsar.ConsumerOptions
	encoding        *component.ID
}

func newTracesReceiver(config Config, set receiver.CreateSettings, nextConsumer consumer.Traces) (*pulsarTracesConsumer, error) {
	options := config.clientOptions()
	client, err := pulsar.NewClient(options)
	if err != nil {
		return nil, err
	}

	consumerOptions, err := config.consumerOptions()
	if err != nil {
		return nil, err
	}

	return &pulsarTracesConsumer{
		tracesConsumer:  nextConsumer,
		topic:           config.Topic,
		encoding:        config.EncodingID,
		settings:        set,
		client:          client,
		consumerOptions: consumerOptions,
	}, nil
}

func (c *pulsarTracesConsumer) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	extension, ok := host.GetExtensions()[*c.encoding]
	if !ok {
		return fmt.Errorf("extension '%s' not found", c.encoding)
	}
	unmarshaler, ok := extension.(encoding.TracesUnmarshalerExtension)
	if !ok {
		return fmt.Errorf(errIncorrectEncoding, c.encoding, "TracesUnmarshaler")
	}
	c.unmarshaler = unmarshaler

	_consumer, err := c.client.Subscribe(c.consumerOptions)
	if err == nil {
		c.consumer = _consumer
		go func() {
			if e := consumerTracesLoop(ctx, c); e != nil {
				c.settings.Logger.Error("consume traces loop occurs an error", zap.Error(e))
			}
		}()
	}

	return err
}

func consumerTracesLoop(ctx context.Context, c *pulsarTracesConsumer) error {
	unmarshaler := c.unmarshaler
	traceConsumer := c.tracesConsumer

	for {
		message, err := c.consumer.Receive(ctx)
		if err != nil {
			if strings.Contains(err.Error(), alreadyClosedError) {
				return err
			}
			if errors.Is(err, context.Canceled) {
				c.settings.Logger.Info("exiting consume traces loop")
				return err
			}
			c.settings.Logger.Error("failed to receive traces message from Pulsar, waiting for one second before retrying", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}

		traces, err := unmarshaler.UnmarshalTraces(message.Payload())
		if err != nil {
			c.settings.Logger.Error("failed to unmarshaler traces message", zap.Error(err))
			c.consumer.Ack(message)
			return err
		}

		if err := traceConsumer.ConsumeTraces(context.Background(), traces); err != nil {
			c.settings.Logger.Error("consume traces failed", zap.Error(err))
		}
		c.consumer.Ack(message)
	}
}

func (c *pulsarTracesConsumer) Shutdown(context.Context) error {
	if c.cancel == nil {
		return nil
	}
	c.cancel()
	c.consumer.Close()
	c.client.Close()
	return nil
}

type pulsarMetricsConsumer struct {
	metricsConsumer consumer.Metrics
	unmarshaler     encoding.MetricsUnmarshalerExtension
	topic           string
	client          pulsar.Client
	consumer        pulsar.Consumer
	cancel          context.CancelFunc
	settings        receiver.CreateSettings
	consumerOptions pulsar.ConsumerOptions
	encoding        *component.ID
}

func newMetricsReceiver(config Config, set receiver.CreateSettings, nextConsumer consumer.Metrics) (*pulsarMetricsConsumer, error) {
	options := config.clientOptions()
	client, err := pulsar.NewClient(options)
	if err != nil {
		return nil, err
	}

	consumerOptions, err := config.consumerOptions()
	if err != nil {
		return nil, err
	}

	return &pulsarMetricsConsumer{
		metricsConsumer: nextConsumer,
		topic:           config.Topic,
		encoding:        config.EncodingID,
		settings:        set,
		client:          client,
		consumerOptions: consumerOptions,
	}, nil
}

func (c *pulsarMetricsConsumer) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	extension, ok := host.GetExtensions()[*c.encoding]
	if !ok {
		return fmt.Errorf("extension '%s' not found", c.encoding)
	}
	unmarshaler, ok := extension.(encoding.MetricsUnmarshalerExtension)
	if !ok {
		return fmt.Errorf(errIncorrectEncoding, c.encoding, "MetricsUnmarshaler")

	}
	c.unmarshaler = unmarshaler

	_consumer, err := c.client.Subscribe(c.consumerOptions)
	if err == nil {
		c.consumer = _consumer

		go func() {
			if e := consumeMetricsLoop(ctx, c); e != nil {
				c.settings.Logger.Error("consume metrics loop occurs an error", zap.Error(e))
			}
		}()
	}

	return err
}

func consumeMetricsLoop(ctx context.Context, c *pulsarMetricsConsumer) error {
	unmarshaler := c.unmarshaler
	metricsConsumer := c.metricsConsumer

	for {
		message, err := c.consumer.Receive(ctx)
		if err != nil {
			if strings.Contains(err.Error(), alreadyClosedError) {
				return err
			}
			if errors.Is(err, context.Canceled) {
				c.settings.Logger.Info("exiting consume metrics loop")
				return err
			}

			c.settings.Logger.Error("failed to receive metrics message from Pulsar, waiting for one second before retrying", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}

		metrics, err := unmarshaler.UnmarshalMetrics(message.Payload())
		if err != nil {
			c.settings.Logger.Error("failed to unmarshaler metrics message", zap.Error(err))
			c.consumer.Ack(message)
			return err
		}

		if err := metricsConsumer.ConsumeMetrics(context.Background(), metrics); err != nil {
			c.settings.Logger.Error("consume traces failed", zap.Error(err))
		}

		c.consumer.Ack(message)
	}
}

func (c *pulsarMetricsConsumer) Shutdown(context.Context) error {
	if c.cancel == nil {
		return nil
	}
	c.cancel()
	c.consumer.Close()
	c.client.Close()
	return nil
}

type pulsarLogsConsumer struct {
	logsConsumer    consumer.Logs
	unmarshaler     encoding.LogsUnmarshalerExtension
	topic           string
	client          pulsar.Client
	consumer        pulsar.Consumer
	cancel          context.CancelFunc
	settings        receiver.CreateSettings
	consumerOptions pulsar.ConsumerOptions
	encoding        *component.ID
}

func newLogsReceiver(config Config, set receiver.CreateSettings, nextConsumer consumer.Logs) (*pulsarLogsConsumer, error) {
	options := config.clientOptions()
	client, err := pulsar.NewClient(options)
	if err != nil {
		return nil, err
	}

	consumerOptions, err := config.consumerOptions()
	if err != nil {
		return nil, err
	}

	return &pulsarLogsConsumer{
		logsConsumer:    nextConsumer,
		topic:           config.Topic,
		encoding:        config.EncodingID,
		settings:        set,
		client:          client,
		consumerOptions: consumerOptions,
	}, nil
}

func (c *pulsarLogsConsumer) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	extension, ok := host.GetExtensions()[*c.encoding]
	if !ok {
		return fmt.Errorf("extension '%s' not found", c.encoding)
	}
	unmarshaler, ok := extension.(encoding.LogsUnmarshalerExtension)
	if !ok {
		return fmt.Errorf(errIncorrectEncoding, c.encoding, "LogsUnmarshaler")
	}
	c.unmarshaler = unmarshaler

	_consumer, err := c.client.Subscribe(c.consumerOptions)
	if err == nil {
		c.consumer = _consumer
		go func() {
			if e := consumeLogsLoop(ctx, c); e != nil {
				c.settings.Logger.Error("consume logs loop occurs an error", zap.Error(e))
			}
		}()
	}

	return err
}

func consumeLogsLoop(ctx context.Context, c *pulsarLogsConsumer) error {
	unmarshaler := c.unmarshaler
	logsConsumer := c.logsConsumer

	for {
		message, err := c.consumer.Receive(ctx)
		if err != nil {
			if strings.Contains(err.Error(), alreadyClosedError) {
				return err
			}
			if errors.Is(err, context.Canceled) {
				c.settings.Logger.Info("exiting consume traces loop canceled")
				return err
			}
			c.settings.Logger.Error("failed to receive logs message from Pulsar, waiting for one second before retrying", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}

		logs, err := unmarshaler.UnmarshalLogs(message.Payload())
		if err != nil {
			c.settings.Logger.Error("failed to unmarshaler logs message", zap.Error(err))
			c.consumer.Ack(message)
			return err
		}

		if err := logsConsumer.ConsumeLogs(context.Background(), logs); err != nil {
			c.settings.Logger.Error("consume traces failed", zap.Error(err))
		}

		c.consumer.Ack(message)
	}
}

func (c *pulsarLogsConsumer) Shutdown(context.Context) error {
	if c.cancel == nil {
		return nil
	}
	c.cancel()
	c.consumer.Close()
	c.client.Close()
	return nil
}
