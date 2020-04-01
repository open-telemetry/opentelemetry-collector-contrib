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
	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/oterr"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	errNilNextConsumer = errors.New("nil nextConsumer")
	factories          = buildFactoryMap(&config.ExampleReceiverFactory{})
)

type factoryMap map[string]component.ReceiverFactoryBase

func (fm factoryMap) get(receiverType string) (component.ReceiverFactoryBase, error) {
	if factory, ok := factories[receiverType]; ok {
		return factory, nil
	}
	return nil, fmt.Errorf("factory does not exist for receiver type %q", receiverType)
}

func buildFactoryMap(factories ...component.ReceiverFactoryBase) factoryMap {
	ret := map[string]component.ReceiverFactoryBase{}
	for _, f := range factories {
		ret[f.Type()] = f
	}
	return ret
}

var _ component.MetricsReceiver = (*receiverCreator)(nil)

// receiverCreator implements consumer.MetricsConsumer.
type receiverCreator struct {
	nextConsumer consumer.MetricsConsumerOld
	logger       *zap.Logger
	cfg          *Config
	receivers    []component.Receiver
}

// New creates the receiver_creator with the given parameters.
func New(logger *zap.Logger, nextConsumer consumer.MetricsConsumerOld, cfg *Config) (component.MetricsReceiver, error) {
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
func (dr *receiverCreator) loadRuntimeReceiverConfig(
	staticSubConfig *subreceiverConfig,
	subConfigFromEnv map[string]interface{}) (configmodels.Receiver, error) {
	// Load config under <receiver>/<id> since loadReceiver and CustomUnmarshaler expects this structure.
	viperConfig := viper.New()
	viperConfig.Set(staticSubConfig.fullName, map[string]interface{}{})
	subreceiverConfig := viperConfig.Sub(staticSubConfig.fullName)

	// Merge in the config values specified in the config file.
	if err := subreceiverConfig.MergeConfigMap(staticSubConfig.config); err != nil {
		return nil, fmt.Errorf("failed to merge subreceiver config from config file: %v", err)
	}

	// Merge in subConfigFromEnv containing values discovered at runtime.
	if err := subreceiverConfig.MergeConfigMap(subConfigFromEnv); err != nil {
		return nil, fmt.Errorf("failed to merge subreceiver config from discovered runtime values: %v", err)
	}

	receiverConfig, err := config.LoadReceiver(staticSubConfig.fullName, viperConfig, factories)
	if err != nil {
		return nil, fmt.Errorf("failed to load subreceiver config: %v", err)
	}
	// Sets dynamically created receiver to something like receiver_creator/1/redis{endpoint="localhost:6380"}.
	// TODO: Need to make sure this is unique (just endpoint is probably not totally sufficient).
	receiverConfig.SetName(fmt.Sprintf("%s/%s{endpoint=%q}", dr.cfg.Name(), staticSubConfig.fullName, subreceiverConfig.GetString("endpoint")))
	return receiverConfig, nil
}

// createRuntimeReceiver creates a receiver that is discovered at runtime.
func (dr *receiverCreator) createRuntimeReceiver(cfg configmodels.Receiver) (component.MetricsReceiver, error) {
	factory, err := factories.get(cfg.Type())
	if err != nil {
		return nil, err
	}
	receiverFactory := factory.(component.ReceiverFactoryOld)
	return receiverFactory.CreateMetricsReceiver(dr.logger, cfg, dr)
}

// Start receiver_creator.
func (dr *receiverCreator) Start(host component.Host) error {
	// TODO: Temporarily load a single instance of all subreceivers for testing.
	// Will be done in reaction to observer events later.
	for _, subconfig := range dr.cfg.subreceiverConfigs {
		cfg, err := dr.loadRuntimeReceiverConfig(subconfig, map[string]interface{}{})
		if err != nil {
			return err
		}
		recvr, err := dr.createRuntimeReceiver(cfg)
		if err != nil {
			return err
		}

		if err := recvr.Start(host); err != nil {
			return fmt.Errorf("failed starting subreceiver %s: %v", cfg.Name(), err)
		}

		dr.receivers = append(dr.receivers, recvr)
	}

	// TODO: Can result in some receivers left running if an error is encountered
	// but starting receivers here is only temporary and will be removed when
	// observer interface added.

	return nil
}

// Shutdown stops the receiver_creator and all its receivers started at runtime.
func (dr *receiverCreator) Shutdown() error {
	var errs []error

	for _, recvr := range dr.receivers {
		if err := recvr.Shutdown(); err != nil {
			// TODO: Should keep track of which receiver the error is associated with
			// but require some restructuring.
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown on %d receivers failed: %v", len(errs), oterr.CombineErrors(errs))
	}

	return nil
}

// ConsumeMetricsData receives metrics from receivers created at runtime to forward on.
func (dr *receiverCreator) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	return dr.nextConsumer.ConsumeMetricsData(ctx, md)
}
