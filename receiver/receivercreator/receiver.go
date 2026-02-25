// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/service/hostcapabilities"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ receiver.Metrics = (*receiverCreator)(nil)

// receiverCreator implements consumer.Metrics.
type receiverCreator struct {
	params              receiver.Settings
	cfg                 *Config
	nextLogsConsumer    consumer.Logs
	nextMetricsConsumer consumer.Metrics
	nextTracesConsumer  consumer.Traces
	observerHandler     *observerHandler
	observables         []observer.Observable
}

func newReceiverCreator(params receiver.Settings, cfg *Config) receiver.Metrics {
	return &receiverCreator{
		params: params,
		cfg:    cfg,
	}
}

// host is an interface that the component.Host passed to receivercreator's Start function must implement
type host interface {
	component.Host
	hostcapabilities.ComponentFactory
}

// Start receiver_creator.
func (rc *receiverCreator) Start(_ context.Context, h component.Host) error {
	rcHost, ok := h.(host)
	if !ok {
		return errors.New("the receivercreator is not compatible with the provided component.host")
	}

	rc.observerHandler = &observerHandler{
		config:                rc.cfg,
		params:                rc.params,
		receiversByEndpointID: receiverMap{},
		nextLogsConsumer:      rc.nextLogsConsumer,
		nextMetricsConsumer:   rc.nextMetricsConsumer,
		nextTracesConsumer:    rc.nextTracesConsumer,
		runner:                newReceiverRunner(rc.params, rcHost),
	}

	observers := map[component.ID]observer.Observable{}

	// Match all configured observables to the extensions that are running.
	for _, watchObserver := range rc.cfg.WatchObservers {
		for cid, ext := range rcHost.GetExtensions() {
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
