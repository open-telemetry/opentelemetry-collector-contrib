// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"errors"

	"go.opentelemetry.io/collector/pipeline"
)

var (
	errNoValidPipeline = errors.New("All provided pipelines return errors")
	errConsumer        = errors.New("Error registering consumer")
)

type consumerProvider[C any] func(...pipeline.ID) (C, error)

// baseFailoverRouter provides the common infrastructure for failover routing
type baseFailoverRouter[C any] struct {
	cfg       *Config
	consumers []C
}

// getConsumerAtIndex returns the consumer at a specific index
func (f *baseFailoverRouter[C]) getConsumerAtIndex(idx int) C {
	return f.consumers[idx]
}

func newBaseFailoverRouter[C any](provider consumerProvider[C], cfg *Config) (*baseFailoverRouter[C], error) {
	consumers := make([]C, 0)
	for _, pipelines := range cfg.PipelinePriority {
		baseConsumer, err := provider(pipelines...)
		if err != nil {
			return nil, errConsumer
		}
		consumers = append(consumers, baseConsumer)
	}

	return &baseFailoverRouter[C]{
		consumers: consumers,
		cfg:       cfg,
	}, nil
}

// For Testing
func (f *baseFailoverRouter[C]) ModifyConsumerAtIndex(idx int, c C) {
	f.consumers[idx] = c
}

func (f *baseFailoverRouter[C]) TestGetConsumerAtIndex(idx int) C {
	return f.consumers[idx]
}
