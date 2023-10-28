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

type pulsarTracesReceiver struct {
	tracesConsumer consumer.Traces
	client         pulsar.Client
	cancel         context.CancelFunc
	consumer       pulsar.Consumer
	unmarshaler    TracesUnmarshaler
	settings       receiver.CreateSettings
	config         Config
}

func newTracesReceiver(config Config, set receiver.CreateSettings, unmarshalers map[string]TracesUnmarshaler, nextConsumer consumer.Traces) (*pulsarTracesReceiver, error) {
	option := config.Trace
	if err := option.validate(); err != nil {
		return nil, err
	}
	unmarshaler := unmarshalers[option.Encoding]
	if nil == unmarshaler {
		return nil, errUnrecognizedEncoding
	}

	return &pulsarTracesReceiver{
		tracesConsumer: nextConsumer,
		unmarshaler:    unmarshaler,
		settings:       set,
		config:         config,
	}, nil
}

func (c *pulsarTracesReceiver) Start(context.Context, component.Host) error {
	client, _consumer, err := c.config.createConsumer(c.config.Log)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.client = client
	c.consumer = _consumer
	go func() {
		if e := consumerTracesLoop(ctx, c); e != nil {
			c.settings.Logger.Error("consume traces loop occurs an error", zap.Error(e))
		}
	}()
	return err
}

func consumerTracesLoop(ctx context.Context, c *pulsarTracesReceiver) error {
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

		traces, err := unmarshaler.Unmarshal(message.Payload())
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

func (c *pulsarTracesReceiver) Shutdown(context.Context) error {
	if c.cancel == nil {
		return nil
	}
	c.cancel()
	c.consumer.Close()
	c.client.Close()
	return nil
}

func createTracesReceiver(_ context.Context, set receiver.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (receiver.Traces, error) {
	c := *(cfg.(*Config))
	r, err := newTracesReceiver(c, set, defaultTracesUnmarshalers(), nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}
