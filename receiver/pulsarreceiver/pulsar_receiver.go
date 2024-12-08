// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

var errUnrecognizedEncoding = errors.New("unrecognized encoding")

const alreadyClosedError = "AlreadyClosedError"

const transport = "pulsar"

type pulsarTracesConsumer struct {
	tracesConsumer  consumer.Traces
	topic           string
	client          pulsar.Client
	cancel          context.CancelFunc
	consumer        pulsar.Consumer
	unmarshaler     TracesUnmarshaler
	settings        receiver.Settings
	consumerOptions pulsar.ConsumerOptions
	obsrecv         *receiverhelper.ObsReport
}

func newTracesReceiver(config Config, set receiver.Settings, unmarshalers map[string]TracesUnmarshaler, nextConsumer consumer.Traces) (*pulsarTracesConsumer, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              transport,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}
	unmarshaler := unmarshalers[config.Encoding]
	if nil == unmarshaler {
		return nil, errUnrecognizedEncoding
	}

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
		obsrecv:         obsrecv,
		tracesConsumer:  nextConsumer,
		topic:           config.Topic,
		unmarshaler:     unmarshaler,
		settings:        set,
		client:          client,
		consumerOptions: consumerOptions,
	}, nil
}

func (c *pulsarTracesConsumer) Start(context.Context, component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

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

	// TODO: Ensure returned errors are handled
	for {
		obsCtx := c.obsrecv.StartTracesOp(ctx)
		message, err := c.consumer.Receive(ctx)
		if err != nil {
			c.obsrecv.EndTracesOp(obsCtx, unmarshaler.Encoding(), 0, err)
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

		traces, err := unmarshaler.Unmarshal(message.Payload())
		if err != nil {
			c.settings.Logger.Error("failed to unmarshaler traces message", zap.Error(err))
			c.obsrecv.EndTracesOp(obsCtx, unmarshaler.Encoding(), 0, err)
			_ = c.consumer.Ack(message)
			return err
		}
		err = traceConsumer.ConsumeTraces(context.Background(), traces)
		if err != nil {
			c.settings.Logger.Error("consume traces failed", zap.Error(err))
		}
		c.obsrecv.EndTracesOp(obsCtx, unmarshaler.Encoding(), traces.SpanCount(), err)
		_ = c.consumer.Ack(message)
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
	unmarshaler     MetricsUnmarshaler
	topic           string
	client          pulsar.Client
	consumer        pulsar.Consumer
	cancel          context.CancelFunc
	settings        receiver.Settings
	consumerOptions pulsar.ConsumerOptions
	obsrecv         *receiverhelper.ObsReport
}

func newMetricsReceiver(config Config, set receiver.Settings, unmarshalers map[string]MetricsUnmarshaler, nextConsumer consumer.Metrics) (*pulsarMetricsConsumer, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              transport,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}
	unmarshaler := unmarshalers[config.Encoding]
	if nil == unmarshaler {
		return nil, errUnrecognizedEncoding
	}

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
		obsrecv:         obsrecv,
		metricsConsumer: nextConsumer,
		topic:           config.Topic,
		unmarshaler:     unmarshaler,
		settings:        set,
		client:          client,
		consumerOptions: consumerOptions,
	}, nil
}

func (c *pulsarMetricsConsumer) Start(context.Context, component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

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

	// TODO: Ensure returned errors are handled
	for {
		obsCtx := c.obsrecv.StartMetricsOp(ctx)
		message, err := c.consumer.Receive(ctx)
		if err != nil {
			c.obsrecv.EndMetricsOp(obsCtx, unmarshaler.Encoding(), 0, err)
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

		metrics, err := unmarshaler.Unmarshal(message.Payload())
		if err != nil {
			c.settings.Logger.Error("failed to unmarshaler metrics message", zap.Error(err))
			c.obsrecv.EndMetricsOp(obsCtx, unmarshaler.Encoding(), 0, err)
			_ = c.consumer.Ack(message)
			return err
		}
		err = metricsConsumer.ConsumeMetrics(context.Background(), metrics)
		if err != nil {
			c.settings.Logger.Error("consume traces failed", zap.Error(err))
		}
		c.obsrecv.EndMetricsOp(obsCtx, unmarshaler.Encoding(), metrics.DataPointCount(), err)

		_ = c.consumer.Ack(message)
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
	unmarshaler     LogsUnmarshaler
	topic           string
	client          pulsar.Client
	consumer        pulsar.Consumer
	cancel          context.CancelFunc
	settings        receiver.Settings
	consumerOptions pulsar.ConsumerOptions
	obsrecv         *receiverhelper.ObsReport
}

func newLogsReceiver(config Config, set receiver.Settings, unmarshalers map[string]LogsUnmarshaler, nextConsumer consumer.Logs) (*pulsarLogsConsumer, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              transport,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}
	unmarshaler := unmarshalers[config.Encoding]
	if nil == unmarshaler {
		return nil, errUnrecognizedEncoding
	}

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
		obsrecv:         obsrecv,
		logsConsumer:    nextConsumer,
		topic:           config.Topic,
		cancel:          nil,
		unmarshaler:     unmarshaler,
		settings:        set,
		client:          client,
		consumerOptions: consumerOptions,
	}, nil
}

func (c *pulsarLogsConsumer) Start(context.Context, component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

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
		obsCtx := c.obsrecv.StartLogsOp(ctx)
		message, err := c.consumer.Receive(ctx)
		if err != nil {
			c.obsrecv.EndLogsOp(obsCtx, unmarshaler.Encoding(), 0, err)
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

		logs, err := unmarshaler.Unmarshal(message.Payload())
		if err != nil {
			c.settings.Logger.Error("failed to unmarshaler logs message", zap.Error(err))
			c.obsrecv.EndLogsOp(obsCtx, unmarshaler.Encoding(), 0, err)
			_ = c.consumer.Ack(message)
			return err
		}
		err = logsConsumer.ConsumeLogs(context.Background(), logs)
		if err != nil {
			c.settings.Logger.Error("consume traces failed", zap.Error(err))
		}
		c.obsrecv.EndLogsOp(obsCtx, unmarshaler.Encoding(), logs.LogRecordCount(), err)
		_ = c.consumer.Ack(message)
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
