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

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/component/componenterror"
	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"go.uber.org/zap"
)

var (
	errNilNextConsumer = errors.New("nil nextConsumer")
)

var _ component.MetricsReceiver = (*receiverCreator)(nil)

// receiverCreator implements consumer.MetricsConsumer.
type receiverCreator struct {
	nextConsumer consumer.MetricsConsumerOld
	logger       *zap.Logger
	cfg          *Config
	receivers    []component.Receiver
}

// new creates the receiver_creator with the given parameters.
func new(logger *zap.Logger, nextConsumer consumer.MetricsConsumerOld, cfg *Config) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextConsumer
	}

	r := &receiverCreator{
		logger:       logger,
		nextConsumer: nextConsumer,
		cfg:          cfg,
	}
	return r, nil
}

// loadRuntimeReceiverConfig loads the given subreceiverConfig merged with config values
// that may have been discovered at runtime.
func (rc *receiverCreator) loadRuntimeReceiverConfig(factory component.ReceiverFactoryOld,
	staticSubConfig *subreceiverConfig, discoveredConfig userConfigMap) (configmodels.Receiver, error) {
	mergedConfig := config.NewViper()

	// Merge in the config values specified in the config file.
	if err := mergedConfig.MergeConfigMap(staticSubConfig.config); err != nil {
		return nil, fmt.Errorf("failed to merge subreceiver config from config file: %v", err)
	}

	// Merge in discoveredConfig containing values discovered at runtime.
	if err := mergedConfig.MergeConfigMap(discoveredConfig); err != nil {
		return nil, fmt.Errorf("failed to merge subreceiver config from discovered runtime values: %v", err)
	}

	viperConfig := config.NewViper()

	// Load config under <receiver>/<id> since loadReceiver and CustomUnmarshaler expects this structure.
	viperConfig.Set(staticSubConfig.fullName, mergedConfig.AllSettings())

	receiverConfig, err := config.LoadReceiver(mergedConfig, staticSubConfig.receiverType, staticSubConfig.fullName, factory)
	if err != nil {
		return nil, fmt.Errorf("failed to load subreceiver config: %v", err)
	}
	// Sets dynamically created receiver to something like receiver_creator/1/redis{endpoint="localhost:6380"}.
	// TODO: Need to make sure this is unique (just endpoint is probably not totally sufficient).
	receiverConfig.SetName(fmt.Sprintf("%s/%s{endpoint=%q}", rc.cfg.Name(), staticSubConfig.fullName, mergedConfig.GetString("endpoint")))
	return receiverConfig, nil
}

// createRuntimeReceiver creates a receiver that is discovered at runtime.
func (rc *receiverCreator) createRuntimeReceiver(factory component.ReceiverFactoryOld, cfg configmodels.Receiver) (component.MetricsReceiver, error) {
	return factory.CreateMetricsReceiver(rc.logger, cfg, rc.nextConsumer)
}

// Start receiver_creator.
func (rc *receiverCreator) Start(ctx context.Context, host component.Host) error {
	// TODO: Temporarily load a single instance of all subreceivers for testing.
	// Will be done in reaction to observer events later.
	for _, subconfig := range rc.cfg.subreceiverConfigs {
		factory := host.GetFactory(component.KindReceiver, subconfig.receiverType)

		if factory == nil {
			rc.logger.Error("unable to lookup factory for receiver", zap.String("receiver", subconfig.receiverType))
			continue
		}

		receiverFactory := factory.(component.ReceiverFactoryOld)

		cfg, err := rc.loadRuntimeReceiverConfig(receiverFactory, subconfig, userConfigMap{})
		if err != nil {
			return err
		}
		recvr, err := rc.createRuntimeReceiver(receiverFactory, cfg)
		if err != nil {
			return err
		}

		if err := recvr.Start(ctx, host); err != nil {
			return fmt.Errorf("failed starting subreceiver %s: %v", cfg.Name(), err)
		}

		rc.receivers = append(rc.receivers, recvr)
	}

	// TODO: Can result in some receivers left running if an error is encountered
	// but starting receivers here is only temporary and will be removed when
	// observer interface added.

	return nil
}

// Shutdown stops the receiver_creator and all its receivers started at runtime.
func (rc *receiverCreator) Shutdown(ctx context.Context) error {
	var errs []error

	for _, recvr := range rc.receivers {
		if err := recvr.Shutdown(ctx); err != nil {
			// TODO: Should keep track of which receiver the error is associated with
			// but require some restructuring.
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown on %d receivers failed: %v", len(errs), componenterror.CombineErrors(errs))
	}

	return nil
}
