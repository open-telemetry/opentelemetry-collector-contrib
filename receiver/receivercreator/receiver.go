// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ receiver.Metrics = (*receiverCreator)(nil)

// receiverCreator implements consumer.Metrics.
type receiverCreator struct {
	params          receiver.CreateSettings
	cfg             *Config
	nextConsumer    consumer.Metrics
	observerHandler *observerHandler
	observables     []observer.Observable
}

// newReceiverCreator creates the receiver_creator with the given parameters.
func newReceiverCreator(params receiver.CreateSettings, cfg *Config, nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	r := &receiverCreator{
		params:       params,
		cfg:          cfg,
		nextConsumer: nextConsumer,
	}
	return r, nil
}

// loggingHost provides a safer version of host that logs errors instead of exiting the process.
type loggingHost struct {
	component.Host
	logger *zap.Logger
}

// ReportFatalError causes a log to be made instead of terminating the process as Host does by default.
func (h *loggingHost) ReportFatalError(err error) {
	h.logger.Error("receiver reported a fatal error", zap.Error(err))
}

var _ component.Host = (*loggingHost)(nil)

// Start receiver_creator.
func (rc *receiverCreator) Start(_ context.Context, host component.Host) error {
	rc.observerHandler = &observerHandler{
		config:                rc.cfg,
		params:                rc.params,
		receiversByEndpointID: receiverMap{},
		nextConsumer:          rc.nextConsumer,
		runner: &receiverRunner{
			params:      rc.params,
			idNamespace: rc.params.ID,
			host:        &loggingHost{host, rc.params.Logger},
		},
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
