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

type pulsarMetricsReceiver struct {
	metricsConsumer consumer.Metrics
	unmarshaler     MetricsUnmarshaler
	client          pulsar.Client
	consumer        pulsar.Consumer
	cancel          context.CancelFunc
	settings        receiver.CreateSettings
	config          Config
}

func newMetricsReceiver(config Config, set receiver.CreateSettings, unmarshalers map[string]MetricsUnmarshaler, nextConsumer consumer.Metrics) (*pulsarMetricsReceiver, error) {
	option := config.Metric
	if err := option.validate(); err != nil {
		return nil, err
	}
	unmarshaler := unmarshalers[option.Encoding]
	if nil == unmarshaler {
		return nil, errUnrecognizedEncoding
	}

	return &pulsarMetricsReceiver{
		metricsConsumer: nextConsumer,
		unmarshaler:     unmarshaler,
		settings:        set,
		config:          config,
	}, nil
}

func (c *pulsarMetricsReceiver) Start(context.Context, component.Host) error {
	client, _consumer, err := c.config.createConsumer(c.config.Log)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.client = client
	c.consumer = _consumer
	go func() {
		if e := consumeMetricsLoop(ctx, c); e != nil {
			c.settings.Logger.Error("consume metrics loop occurs an error", zap.Error(e))
		}
	}()
	return err
}

func consumeMetricsLoop(ctx context.Context, c *pulsarMetricsReceiver) error {
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

		metrics, err := unmarshaler.Unmarshal(message.Payload())
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

func (c *pulsarMetricsReceiver) Shutdown(context.Context) error {
	if c.cancel == nil {
		return nil
	}
	c.cancel()
	c.consumer.Close()
	c.client.Close()
	return nil
}

func createMetricsReceiver(_ context.Context, set receiver.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	c := *(cfg.(*Config))
	r, err := newMetricsReceiver(c, set, defaultMetricsUnmarshalers(), nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}
