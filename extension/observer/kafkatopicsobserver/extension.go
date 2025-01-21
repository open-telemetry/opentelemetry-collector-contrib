// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// Mock implementation for the initial PR
var (
	_ extension.Extension      = (*kafkaTopicsObserver)(nil)
	_ observer.EndpointsLister = (*kafkaTopicsObserver)(nil)
	_ observer.Observable      = (*kafkaTopicsObserver)(nil)
)

type kafkaTopicsObserver struct {
	*observer.EndpointsWatcher
	logger *zap.Logger
	config *Config
	cancel func()
	once   *sync.Once
	ctx    context.Context
}

func newObserver(logger *zap.Logger, config *Config) (extension.Extension, error) {
	d := &kafkaTopicsObserver{
		logger: logger, config: config,
		once: &sync.Once{},
		cancel: func() {
		},
	}
	d.EndpointsWatcher = observer.NewEndpointsWatcher(d, time.Second, logger)
	return d, nil
}

func (d *kafkaTopicsObserver) ListEndpoints() []observer.Endpoint {
	var endpoints []observer.Endpoint
	return endpoints
}

func (d *kafkaTopicsObserver) Start(ctx context.Context, _ component.Host) error {
	return nil
}

func (d *kafkaTopicsObserver) Shutdown(_ context.Context) error {
	d.StopListAndWatch()
	d.cancel()
	return nil
}
