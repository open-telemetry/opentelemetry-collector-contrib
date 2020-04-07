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

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configerror"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// This file implements factory for receiver_creator. A receiver_creator can create other receivers at runtime.

const (
	typeStr = "receiver_creator"
)

// Factory is the factory for receiver_creator.
type Factory struct {
}

var _ component.ReceiverFactoryOld = (*Factory)(nil)

// Type gets the type of the Receiver config created by this factory.
func (f *Factory) Type() string {
	return typeStr
}

// CustomUnmarshaler returns custom unmarshaler for receiver_creator config.
func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return func(sourceViperSection *viper.Viper, intoCfg interface{}) error {
		if sourceViperSection == nil {
			// Nothing to do if there is no config given.
			return nil
		}
		c := intoCfg.(*Config)

		// TODO: Change the sub-receivers to be under "receivers" to allow other config for the main receiver-creator receiver.
		for subreceiverKey := range sourceViperSection.AllSettings() {
			cfgSection := sourceViperSection.GetStringMap(subreceiverKey + "::config")
			subreceiver, err := newSubreceiverConfig(subreceiverKey, cfgSection)
			if err != nil {
				return err
			}

			// Unmarshals receiver_creator configuration like rule.
			// TODO: validate discovery rule
			if err := sourceViperSection.UnmarshalKey(subreceiverKey, &subreceiver); err != nil {
				return fmt.Errorf("failed to deserialize sub-receiver %q: %s", subreceiverKey, err)
			}

			c.subreceiverConfigs[subreceiverKey] = subreceiver
		}

		return nil
	}
}

// CreateDefaultConfig creates the default configuration for receiver_creator.
func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		subreceiverConfigs: map[string]*subreceiverConfig{},
	}
}

// CreateTraceReceiver creates a trace receiver based on provided config.
func (f *Factory) CreateTraceReceiver(context.Context, *zap.Logger, configmodels.Receiver,
	consumer.TraceConsumerOld) (component.TraceReceiver, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsReceiver creates a metrics receiver based on provided config.
func (f *Factory) CreateMetricsReceiver(
	logger *zap.Logger,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {
	return new(logger, consumer, cfg.(*Config))
}
