// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ receiver.Metrics = (*receiverCreator)(nil)

// receiverCreator implements consumer.Metrics.
type receiverCreator struct {
	params              receiver.CreateSettings
	cfg                 *Config
	nextLogsConsumer    consumer.Logs
	nextMetricsConsumer consumer.Metrics
	nextTracesConsumer  consumer.Traces
	observerHandler     *observerHandler
	observables         []observer.Observable
}

// newLogsReceiverCreator creates the receiver_creator with the given parameters.
func newLogsReceiverCreator(params receiver.CreateSettings, cfg *Config, nextConsumer consumer.Logs) (receiver.Logs, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	r := &receiverCreator{
		params:           params,
		cfg:              cfg,
		nextLogsConsumer: nextConsumer,
	}
	return r, nil
}

// newMetricsReceiverCreator creates the receiver_creator with the given parameters.
func newMetricsReceiverCreator(params receiver.CreateSettings, cfg *Config, nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	r := &receiverCreator{
		params:              params,
		cfg:                 cfg,
		nextMetricsConsumer: nextConsumer,
	}
	return r, nil
}

// newTracesReceiverCreator creates the receiver_creator with the given parameters.
func newTracesReceiverCreator(params receiver.CreateSettings, cfg *Config, nextConsumer consumer.Traces) (receiver.Traces, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	r := &receiverCreator{
		params:             params,
		cfg:                cfg,
		nextTracesConsumer: nextConsumer,
	}
	return r, nil
}

// Start receiver_creator.
func (rc *receiverCreator) Start(_ context.Context, host component.Host) error {
	rc.observerHandler = &observerHandler{
		config:                rc.cfg,
		params:                rc.params,
		receiversByEndpointID: receiverMap{},
		nextLogsConsumer:      rc.nextLogsConsumer,
		nextMetricsConsumer:   rc.nextMetricsConsumer,
		nextTracesConsumer:    rc.nextTracesConsumer,
		runner:                newReceiverRunner(rc.params, host),
	}

	observers := map[component.ID]observer.Observable{}

	// Match all configured observables to the extensions that are running.
	for _, watchObserver := range rc.cfg.WatchObservers {
		for cid, ext := range host.GetExtensions() {
			if cid != watchObserver {
				continue
			}

			obs, ok := ext.(observer.Observable)
			if !ok {
				return fmt.Errorf("extension %q in watch_observers is not an observer", watchObserver.String())
			}
			observers[watchObserver] = obs
		}
	}

	// Make sure all observables are present before starting any.
	for _, watchObserver := range rc.cfg.WatchObservers {
		if observers[watchObserver] == nil {
			return fmt.Errorf("failed to find observer %q in the extensions list", watchObserver.String())
		}
	}

	if len(observers) == 0 {
		rc.params.Logger.Warn("no observers were configured and no subreceivers will be started. receiver_creator will be disabled")
	}

	// Start all configured watchers.
	for _, observable := range observers {
		rc.observables = append(rc.observables, observable)
		observable.ListAndWatch(rc.observerHandler)
	}

	return nil
}

// Shutdown stops the receiver_creator and all its receivers started at runtime.
func (rc *receiverCreator) Shutdown(context.Context) error {
	for _, observable := range rc.observables {
		observable.Unsubscribe(rc.observerHandler)
	}
	if rc.observerHandler == nil {
		return nil
	}
	return rc.observerHandler.shutdown()
}
