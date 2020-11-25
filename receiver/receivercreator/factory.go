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
	"reflect"

	"github.com/spf13/viper"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// This file implements factory for receiver_creator. A receiver_creator can create other receivers at runtime.

const (
	typeStr = "receiver_creator"
)

// NewFactory creates a factory for receiver creator.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithCustomUnmarshaler(customUnmarshaler),
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func createDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		receiverTemplates: map[string]receiverTemplate{},
	}
}

func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	return newReceiverCreator(params, cfg.(*Config), consumer)
}

func customUnmarshaler(sourceViperSection *viper.Viper, intoCfg interface{}) error {
	if sourceViperSection == nil {
		// Nothing to do if there is no config given.
		return nil
	}
	c := intoCfg.(*Config)

	if err := sourceViperSection.Unmarshal(&c); err != nil {
		return err
	}

	receiversCfg, err := config.ViperSubExact(sourceViperSection, receiversConfigKey)
	if err != nil {
		return fmt.Errorf("failed to create sub viper for key %q", receiversConfigKey)
	}

	for subreceiverKey := range receiversCfg.AllSettings() {
		subViperCfg, err := config.ViperSubExact(receiversCfg, subreceiverKey)
		cfgSection := subViperCfg.GetStringMap(configKey)
		if err != nil {
			return fmt.Errorf("failed to create sub viper for key %q.%q", receiversConfigKey, configKey)
		}
		subreceiver, err := newReceiverTemplate(subreceiverKey, cfgSection)
		if err != nil {
			return err
		}

		// Unmarshals receiver_creator configuration like rule.
		if err = receiversCfg.UnmarshalKey(subreceiverKey, &subreceiver); err != nil {
			return fmt.Errorf("failed to deserialize sub-receiver %q: %s", subreceiverKey, err)
		}

		if !observer.EndpointTypes[subreceiver.Type] {
			return fmt.Errorf("subreceiver type must be one of: %v", reflect.ValueOf(observer.EndpointTypes).MapKeys())
		}

		subreceiver.rule, err = newRule(subreceiver.Rule)
		if err != nil {
			return fmt.Errorf("subreceiver %q rule is invalid: %v", subreceiverKey, err)
		}

		c.receiverTemplates[subreceiverKey] = subreceiver
	}

	return nil
}
