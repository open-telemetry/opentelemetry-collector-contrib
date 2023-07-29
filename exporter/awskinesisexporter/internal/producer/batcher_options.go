// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package producer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/producer"

import (
	"errors"

	"go.uber.org/zap"
)

type BatcherOptions func(*batcher) error

// WithLogger sets the provided logger for the Batcher
func WithLogger(l *zap.Logger) BatcherOptions {
	return func(p *batcher) error {
		if l == nil {
			return errors.New("nil logger trying to be assigned")
		}
		p.log = l
		return nil
	}
}
