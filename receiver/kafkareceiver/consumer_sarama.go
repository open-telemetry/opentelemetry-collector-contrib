// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func newSaramaConsumer(config *Config, set receiver.Settings, topics []string,
	newConsumeFn newConsumeMessageFunc,
) (*saramaConsumer, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return &saramaConsumer{
		config:           config,
		topics:           topics,
		newConsumeFn:     newConsumeFn,
		settings:         set,
		telemetryBuilder: telemetryBuilder,
	}, nil
}

// saramaConsumer consumes messages from a set of Kafka topics,
// decodes telemetry data using a given unmarshaler, and passes
// them to a consumer.
type saramaConsumer struct {
	config           *Config
	topics           []string
	settings         receiver.Settings
	telemetryBuilder *metadata.TelemetryBuilder
	newConsumeFn     newConsumeMessageFunc

	mu                sync.Mutex
	started           bool
	shutdown          bool
	consumeLoopClosed chan struct{}
	consumerGroup     sarama.ConsumerGroup
}

func (c *saramaConsumer) Start(_ context.Context, host component.Host) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.shutdown {
		return errors.New("kafka consumer already shut down")
	}
	if c.started {
		return errors.New("kafka consumer already started")
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             c.settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: c.settings,
	})
	if err != nil {
		return err
	}

	consumerGroup, err := kafka.NewSaramaConsumerGroup(
		context.Background(),
		c.config.ClientConfig,
		c.config.ConsumerConfig,
	)
	if err != nil {
		return err
	}
	c.consumerGroup = consumerGroup

	handler := &consumerGroupHandler{
		id:                c.settings.ID,
		logger:            c.settings.Logger,
		ready:             make(chan bool),
		obsrecv:           obsrecv,
		autocommitEnabled: c.config.AutoCommit.Enable,
		messageMarking:    c.config.MessageMarking,
		telemetryBuilder:  c.telemetryBuilder,
		backOff:           newExponentialBackOff(c.config.ErrorBackOff),
	}
	consumeMessage, err := c.newConsumeFn(host, obsrecv, c.telemetryBuilder)
	if err != nil {
		return err
	}
	handler.consumeMessage = consumeMessage

	c.consumeLoopClosed = make(chan struct{})
	c.started = true
	go c.consumeLoop(handler)
	return nil
}

func (c *saramaConsumer) consumeLoop(handler sarama.ConsumerGroupHandler) {
	defer close(c.consumeLoopClosed)

	ctx := context.Background()
	for {
		// `Consume` should be called inside an infinite loop, when a
		// server-side rebalance happens, the consumer session will need to be
		// recreated to get the new claims
		if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
			if errors.Is(err, sarama.ErrClosedConsumerGroup) {
				c.settings.Logger.Info("Consumer stopped", zap.Error(ctx.Err()))
				return
			}
			c.settings.Logger.Error("Error from consumer", zap.Error(err))
		}
	}
}

func (c *saramaConsumer) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.shutdown {
		return nil
	}
	c.shutdown = true
	if !c.started {
		return nil
	}

	if err := c.consumerGroup.Close(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.consumeLoopClosed:
	}
	return nil
}

type consumerGroupHandler struct {
	id             component.ID
	consumeMessage consumeMessageFunc
	ready          chan bool
	readyCloser    sync.Once
	logger         *zap.Logger

	obsrecv          *receiverhelper.ObsReport
	telemetryBuilder *metadata.TelemetryBuilder

	autocommitEnabled bool
	messageMarking    MessageMarking
	backOff           *backoff.ExponentialBackOff
	backOffMutex      sync.Mutex
}

func (c *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	c.readyCloser.Do(func() { close(c.ready) })
	c.telemetryBuilder.KafkaReceiverPartitionStart.Add(
		session.Context(), 1, metric.WithAttributes(attribute.String(attrInstanceName, c.id.Name())),
	)
	return nil
}

func (c *consumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	c.telemetryBuilder.KafkaReceiverPartitionClose.Add(
		session.Context(), 1, metric.WithAttributes(attribute.String(attrInstanceName, c.id.Name())),
	)
	return nil
}

func (c *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.logger.Info("Starting consumer group", zap.Int32("partition", claim.Partition()))
	if !c.autocommitEnabled {
		defer session.Commit()
	}
	for {
		select {
		case <-session.Context().Done():
			// Should return when the session's context is canceled.
			//
			// If we do not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout`
			// when rebalancing. See: https://github.com/IBM/sarama/issues/1192
			return nil
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			if err := c.handleMessage(session, claim, message); err != nil {
				return err
			}
		}
	}
}

func (c *consumerGroupHandler) handleMessage(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
	message *sarama.ConsumerMessage,
) error {
	if !c.messageMarking.After {
		session.MarkMessage(message, "")
	}

	attrs := attribute.NewSet(
		attribute.String(attrInstanceName, c.id.String()),
		attribute.String(attrTopic, message.Topic),
		attribute.String(attrPartition, strconv.Itoa(int(claim.Partition()))),
	)
	c.telemetryBuilder.KafkaReceiverOffsetLag.Record(context.Background(),
		claim.HighWaterMarkOffset()-message.Offset-1, metric.WithAttributeSet(attrs),
	)
	msg := wrapSaramaMsg(message)
	if err := c.consumeMessage(session.Context(), msg, attrs); err != nil {
		if c.backOff != nil && !consumererror.IsPermanent(err) {
			backOffDelay := c.getNextBackoff()
			if backOffDelay != backoff.Stop {
				c.logger.Info("Backing off due to error from the next consumer.",
					zap.Error(err),
					zap.Duration("delay", backOffDelay),
					zap.String("topic", message.Topic),
					zap.Int32("partition", claim.Partition()))
				select {
				case <-session.Context().Done():
					return nil
				case <-time.After(backOffDelay):
					if !c.messageMarking.After {
						// Unmark the message so it can be retried
						session.ResetOffset(claim.Topic(), claim.Partition(), message.Offset, "")
					}
					return err
				}
			}
			c.logger.Warn("Stop error backoff because the configured max_elapsed_time is reached",
				zap.Duration("max_elapsed_time", c.backOff.MaxElapsedTime))
		}
		if c.messageMarking.After && !c.messageMarking.OnError {
			// Only return an error if messages are marked after successful processing.
			return err
		}
		// We're either marking messages as consumed ahead of time (disregarding outcome),
		// or after processing but including errors. Either way we should not return an error,
		// as that will restart the consumer unnecessarily.
		c.logger.Error("failed to consume message, skipping due to message_marking config",
			zap.Error(err),
			zap.String("topic", message.Topic),
			zap.Int32("partition", claim.Partition()),
			zap.Int64("offset", message.Offset),
		)
	}
	if c.backOff != nil {
		c.resetBackoff()
	}
	if c.messageMarking.After {
		session.MarkMessage(message, "")
	}
	if !c.autocommitEnabled {
		session.Commit()
	}
	return nil
}

func (c *consumerGroupHandler) getNextBackoff() time.Duration {
	c.backOffMutex.Lock()
	defer c.backOffMutex.Unlock()
	return c.backOff.NextBackOff()
}

func (c *consumerGroupHandler) resetBackoff() {
	c.backOffMutex.Lock()
	defer c.backOffMutex.Unlock()
	c.backOff.Reset()
}
