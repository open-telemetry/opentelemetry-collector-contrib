// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal/metadata"
)

var receivers = sharedcomponent.NewSharedComponents()

func createTracesReceiver(_ context.Context, params receiver.Settings, cfg component.Config, consumer consumer.Traces) (receiver.Traces, error) {
	// check that the configuration is valid
	conf, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotValid
	}

	r := receivers.GetOrAdd(cfg, func() component.Component {
		receiver, err := newGitLabReceiver(params, conf)
		if err != nil {
			// Return a component that will fail on start
			return &errorComponent{err: err}
		}
		return receiver
	})

	receiver := r.Unwrap()
	if errComp, ok := receiver.(*errorComponent); ok {
		return nil, errComp.err
	}

	receiver.(*gitlabReceiver).traceConsumer = consumer
	return r, nil
}

func createMetricsReceiver(_ context.Context, params receiver.Settings, cfg component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	// check that the configuration is valid
	conf, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotValid
	}

	r := receivers.GetOrAdd(cfg, func() component.Component {
		receiver, err := newGitLabReceiver(params, conf)
		if err != nil {
			// Return a component that will fail on start
			return &errorComponent{err: err}
		}
		return receiver
	})

	receiver := r.Unwrap()
	if errComp, ok := receiver.(*errorComponent); ok {
		return nil, errComp.err
	}

	receiver.(*gitlabReceiver).metricConsumer = consumer
	return r, nil
}

// errorComponent is a placeholder component that returns an error
// Used when receiver creation fails
type errorComponent struct {
	err error
}

func (e *errorComponent) Start(context.Context, component.Host) error {
	return e.err
}

func (e *errorComponent) Shutdown(context.Context) error {
	return nil
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}
