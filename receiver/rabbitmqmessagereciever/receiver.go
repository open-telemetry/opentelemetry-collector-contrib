// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqmessagereciever // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqmessagereciever"

import (
	"context"
	"crypto/tls"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqmessagereciever/internal/subscriber"
)

type tlsFactory = func(context.Context) (*tls.Config, error)

type subscriberFactory = func(context.Context) (subscriber.Subscriber, error)

var _ receiver.Metrics = (*rabbitMQMessageReceiver)(nil)

func newRabbitMQReceiver(
	cfg *Config,
	set component.TelemetrySettings,
	subscriberFactory subscriberFactory,
	tlsFactory tlsFactory,
) (*rabbitMQMessageReceiver, error) {
	return &rabbitMQMessageReceiver{}, nil
}

type rabbitMQMessageReceiver struct{}

func (r *rabbitMQMessageReceiver) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (r *rabbitMQMessageReceiver) Shutdown(ctx context.Context) error {
	return nil
}
