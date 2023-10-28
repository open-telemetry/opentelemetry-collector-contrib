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
	"go.uber.org/zap"
)

const alreadyClosedError = "AlreadyClosedError"

type pulsarLogsReceiver struct {
	logsConsumer consumer.Logs
	unmarshaler  LogsUnmarshaler
	client       pulsar.Client
	consumer     pulsar.Consumer
	cancel       context.CancelFunc
	settings     receiver.CreateSettings
	config       Config
}

func newLogsReceiver(config Config, set receiver.CreateSettings, unmarshalers map[string]LogsUnmarshaler, nextConsumer consumer.Logs) (*pulsarLogsReceiver, error) {
	option := config.Log
	if err := option.validate(); err != nil {
		return nil, err
	}
	unmarshaler := unmarshalers[option.Encoding]
	if nil == unmarshaler {
		return nil, errUnrecognizedEncoding
	}
	return &pulsarLogsReceiver{
		config:       config,
		logsConsumer: nextConsumer,
		cancel:       nil,
		unmarshaler:  unmarshaler,
		settings:     set,
	}, nil
}

func (c *pulsarLogsReceiver) Start(context.Context, component.Host) error {
	client, _consumer, err := c.config.createConsumer(c.config.Log)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.client = client
	c.consumer = _consumer
	go func() {
		if e := consumeLogsLoop(ctx, c); e != nil {
			c.settings.Logger.Error("consume logs loop occurs an error", zap.Error(e))
		}
	}()
	return err
}

func consumeLogsLoop(ctx context.Context, c *pulsarLogsReceiver) error {
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

		logs, err := unmarshaler.Unmarshal(message.Payload())
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

func (c *pulsarLogsReceiver) Shutdown(context.Context) error {
	if c.cancel == nil {
		return nil
	}
	c.cancel()
	c.consumer.Close()
	c.client.Close()
	return nil
}

func createLogsReceiver(_ context.Context, set receiver.CreateSettings, cfg component.Config, nextConsumer consumer.Logs) (receiver.Logs, error) {
	c := *(cfg.(*Config))
	r, err := newLogsReceiver(c, set, defaultLogsUnmarshalers(), nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}
