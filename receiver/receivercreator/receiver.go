// Copyright 2020, OpenTelemetry Authors
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

package receivercreator

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var (
	errNilNextConsumer = errors.New("nil nextConsumer")
)

var _ component.MetricsReceiver = (*receiverCreator)(nil)

// receiverCreator implements consumer.MetricsConsumer.
type receiverCreator struct {
	params          component.ReceiverCreateParams
	cfg             *Config
	nextConsumer    consumer.MetricsConsumer
	observerHandler observerHandler
}

// newReceiverCreator creates the receiver_creator with the given parameters.
func newReceiverCreator(params component.ReceiverCreateParams, cfg *Config, nextConsumer consumer.MetricsConsumer) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextConsumer
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
	rc.observerHandler = observerHandler{
		config:                rc.cfg,
		logger:                rc.params.Logger,
		receiversByEndpointID: receiverMap{},
		nextConsumer:          rc.nextConsumer,
		runner: &receiverRunner{
			params:      rc.params,
			idNamespace: rc.cfg.Name(),
			host:        &loggingHost{host, rc.params.Logger},
		}}

	observers := map[configmodels.Type]observer.Observable{}

	// Match all configured observers to the extensions that are running.
	for _, watchObserver := range rc.cfg.WatchObservers {
		for cfg, ext := range host.GetExtensions() {
			if cfg.Type() != watchObserver {
				continue
			}

			obs, ok := ext.(observer.Observable)
			if !ok {
				return fmt.Errorf("extension %q in watch_observers is not an observer", watchObserver)
			}
			observers[watchObserver] = obs
		}
	}

	// Make sure all observers are present before starting any.
	for _, watchObserver := range rc.cfg.WatchObservers {
		if observers[watchObserver] == nil {
			return fmt.Errorf("failed to find observer %q in the extensions list", watchObserver)
		}
	}

	if len(observers) == 0 {
		rc.params.Logger.Warn("no observers were configured and no subreceivers will be started. receiver_creator will be disabled",
			zap.String("receiver", rc.cfg.Name()))
	}

	// Start all configured watchers.
	for _, observable := range observers {
		observable.ListAndWatch(&rc.observerHandler)
	}

	return nil
}

// Shutdown stops the receiver_creator and all its receivers started at runtime.
func (rc *receiverCreator) Shutdown(context.Context) error {
	return rc.observerHandler.shutdown()
}
