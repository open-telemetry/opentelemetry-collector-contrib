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
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
)

// runner starts and stops receiver instances.
type runner interface {
	// start a receiver instance from its static config and discovered config.
	start(receiver receiverConfig, discoveredConfig userConfigMap, nextConsumer consumer.MetricsConsumer) (component.Receiver, error)
	// shutdown a receiver.
	shutdown(rcvr component.Receiver) error
}

// receiverRunner handles starting/stopping of a concrete subreceiver instance.
type receiverRunner struct {
	params      component.ReceiverCreateParams
	idNamespace string
	host        component.Host
}

var _ runner = (*receiverRunner)(nil)

// start a receiver instance from its static config and discovered config.
func (run *receiverRunner) start(
	receiver receiverConfig,
	discoveredConfig userConfigMap,
	nextConsumer consumer.MetricsConsumer,
) (component.Receiver, error) {
	factory := run.host.GetFactory(component.KindReceiver, receiver.typeStr)

	if factory == nil {
		return nil, fmt.Errorf("unable to lookup factory for receiver %q", receiver.typeStr)
	}

	receiverFactory := factory.(component.ReceiverFactory)

	cfg, err := run.loadRuntimeReceiverConfig(receiverFactory, receiver, discoveredConfig)
	if err != nil {
		return nil, err
	}
	recvr, err := run.createRuntimeReceiver(receiverFactory, cfg, nextConsumer)
	if err != nil {
		return nil, err
	}

	if err := recvr.Start(context.Background(), run.host); err != nil {
		return nil, fmt.Errorf("failed starting receiver %s: %v", cfg.Name(), err)
	}

	return recvr, nil
}

// shutdown the given receiver.
func (run *receiverRunner) shutdown(rcvr component.Receiver) error {
	return rcvr.Shutdown(context.Background())
}

// loadRuntimeReceiverConfig loads the given receiverTemplate merged with config values
// that may have been discovered at runtime.
func (run *receiverRunner) loadRuntimeReceiverConfig(
	factory component.ReceiverFactory,
	receiver receiverConfig,
	discoveredConfig userConfigMap,
) (configmodels.Receiver, error) {
	mergedConfig := config.NewViper()

	// Merge in the config values specified in the config file.
	if err := mergedConfig.MergeConfigMap(receiver.config); err != nil {
		return nil, fmt.Errorf("failed to merge template config from config file: %v", err)
	}

	// Merge in discoveredConfig containing values discovered at runtime.
	if err := mergedConfig.MergeConfigMap(discoveredConfig); err != nil {
		return nil, fmt.Errorf("failed to merge template config from discovered runtime values: %v", err)
	}

	receiverConfig, err := config.LoadReceiver(mergedConfig, receiver.typeStr, receiver.fullName, factory)
	if err != nil {
		return nil, fmt.Errorf("failed to load template config: %v", err)
	}
	// Sets dynamically created receiver to something like receiver_creator/1/redis{endpoint="localhost:6380"}.
	// TODO: Need to make sure this is unique (just endpoint is probably not totally sufficient).
	receiverConfig.SetName(fmt.Sprintf("%s/%s{endpoint=%q}", run.idNamespace, receiver.fullName, mergedConfig.GetString(endpointConfigKey)))
	return receiverConfig, nil
}

// createRuntimeReceiver creates a receiver that is discovered at runtime.
func (run *receiverRunner) createRuntimeReceiver(
	factory component.ReceiverFactory,
	cfg configmodels.Receiver,
	nextConsumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	return factory.CreateMetricsReceiver(context.Background(), run.params, cfg, nextConsumer)
}
