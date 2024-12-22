// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriber // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqmessagereciever/internal/subscriber"

import (
	"context"
	"io"
)

type Subscriber interface {
	Subscribe(ctx context.Context, channel string) (chan<- io.Reader, error)
	Close() error
}
