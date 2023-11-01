// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"go.opentelemetry.io/collector/component"
)

type consumerProvider[C any] func(...component.ID) (C, error)

type failoverRouter[C any] struct {
	consumerProvider consumerProvider[C]
	cfg              *Config
}

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config) *failoverRouter[C] {
	return &failoverRouter[C]{
		consumerProvider: provider,
		cfg:              cfg,
	}
}
