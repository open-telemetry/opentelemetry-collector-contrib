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

	"github.com/spf13/cast"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// runner starts and stops receiver instances.
type runner interface {
	// start a receiver instance from its static config and discovered config.
	start(receiver receiverConfig, discoveredConfig userConfigMap, nextConsumer consumer.Metrics) (component.Component, error)
	// shutdown a receiver.
	shutdown(rcvr component.Component) error
}

// receiverRunner handles starting/stopping of a concrete subreceiver instance.
type receiverRunner struct {
	params      rcvr.CreateSettings
	idNamespace component.ID
	host        component.Host
}

var _ runner = (*receiverRunner)(nil)

// start a receiver instance from its static config and discovered config.
func (run *receiverRunner) start(
	receiver receiverConfig,
	discoveredConfig userConfigMap,
	nextConsumer consumer.Metrics,
) (component.Component, error) {
	factory := run.host.GetFactory(component.KindReceiver, receiver.id.Type())

	if factory == nil {
		return nil, fmt.Errorf("unable to lookup factory for receiver %q", receiver.id.String())
	}

	receiverFactory := factory.(rcvr.Factory)

	cfg, targetEndpoint, err := run.loadRuntimeReceiverConfig(receiverFactory, receiver, discoveredConfig)
	if err != nil {
		return nil, err
	}

	// Sets dynamically created receiver to something like receiver_creator/1/redis{endpoint="localhost:6380"}/<EndpointID>.
	id := component.NewIDWithName(factory.Type(), fmt.Sprintf("%s/%s{endpoint=%q}/%s", receiver.id.Name(), run.idNamespace, targetEndpoint, receiver.endpointID))

	recvr, err := run.createRuntimeReceiver(receiverFactory, id, cfg, nextConsumer)
	if err != nil {
		return nil, err
	}

	if err = recvr.Start(context.Background(), run.host); err != nil {
		return nil, err
	}

	return recvr, nil
}

// shutdown the given receiver.
func (run *receiverRunner) shutdown(rcvr component.Component) error {
	return rcvr.Shutdown(context.Background())
}

// loadRuntimeReceiverConfig loads the given receiverTemplate merged with config values
// that may have been discovered at runtime.
func (run *receiverRunner) loadRuntimeReceiverConfig(
	factory rcvr.Factory,
	receiver receiverConfig,
	discoveredConfig userConfigMap,
) (component.Config, string, error) {
	// remove dynamically added "endpoint" field if not supported by receiver
	mergedConfig, targetEndpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, receiver.config, discoveredConfig)
	if err != nil {
		return nil, targetEndpoint, fmt.Errorf("failed to merge constituent template configs: %w", err)
	}

	receiverCfg := factory.CreateDefaultConfig()
	if err := component.UnmarshalConfig(mergedConfig, receiverCfg); err != nil {
		return nil, "", fmt.Errorf("failed to load %q template config: %w", receiver.id.String(), err)
	}
	return receiverCfg, targetEndpoint, nil
}

// mergeTemplateAndDiscoveredConfigs will unify the templated and discovered configs,
// setting the `endpoint` field from the discovered one if 1. not specified by the user
// and 2. determined to be supported (by trial and error of unmarshalling a temp intermediary).
func mergeTemplatedAndDiscoveredConfigs(factory rcvr.Factory, templated, discovered userConfigMap) (*confmap.Conf, string, error) {
	targetEndpoint := cast.ToString(templated[endpointConfigKey])
	if _, endpointSet := discovered[tmpSetEndpointConfigKey]; endpointSet {
		delete(discovered, tmpSetEndpointConfigKey)
		targetEndpoint = cast.ToString(discovered[endpointConfigKey])

		// confirm the endpoint we've added is supported, removing if not
		endpointConfig := confmap.NewFromStringMap(map[string]any{
			endpointConfigKey: targetEndpoint,
		})
		if err := endpointConfig.Unmarshal(factory.CreateDefaultConfig(), confmap.WithErrorUnused()); err != nil {
			// rather than attach to error content that can change over time,
			// confirm the error only arises w/ ErrorUnused mapstructure setting ("invalid keys")
			if err = endpointConfig.Unmarshal(factory.CreateDefaultConfig()); err == nil {
				delete(discovered, endpointConfigKey)
			}
		}
	}
	discoveredConfig := confmap.NewFromStringMap(discovered)
	templatedConfig := confmap.NewFromStringMap(templated)

	// Merge in discoveredConfig containing values discovered at runtime.
	if err := templatedConfig.Merge(discoveredConfig); err != nil {
		return nil, targetEndpoint, fmt.Errorf("failed to merge template config from discovered runtime values: %w", err)
	}
	return templatedConfig, targetEndpoint, nil
}

// createRuntimeReceiver creates a receiver that is discovered at runtime.
func (run *receiverRunner) createRuntimeReceiver(
	factory rcvr.Factory,
	id component.ID,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (rcvr.Metrics, error) {
	runParams := run.params
	runParams.Logger = runParams.Logger.With(zap.String("name", id.String()))
	runParams.ID = id
	return factory.CreateMetricsReceiver(context.Background(), runParams, cfg, nextConsumer)
}
