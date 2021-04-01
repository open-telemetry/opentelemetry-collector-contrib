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

	"github.com/spf13/cast"
	"github.com/spf13/viper"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/translator/conventions"

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

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.ReceiverSettings{
			TypeVal: config.Type(typeStr),
			NameVal: typeStr,
		},
		ResourceAttributes: resourceAttributes{
			observer.PodType: map[string]string{
				conventions.AttributeK8sPod:       "`name`",
				conventions.AttributeK8sPodUID:    "`uid`",
				conventions.AttributeK8sNamespace: "`namespace`",
			},
			observer.PortType: map[string]string{
				conventions.AttributeK8sPod:       "`pod.name`",
				conventions.AttributeK8sPodUID:    "`pod.uid`",
				conventions.AttributeK8sNamespace: "`pod.namespace`",
			},
		},
		receiverTemplates: map[string]receiverTemplate{},
	}
}

func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateParams,
	cfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	return newReceiverCreator(params, cfg.(*Config), consumer)
}

func customUnmarshaler(sourceViperSection *viper.Viper, intoCfg interface{}) error {
	if sourceViperSection == nil {
		// Nothing to do if there is no config given.
		return nil
	}
	cp := config.ParserFromViper(sourceViperSection)
	c := intoCfg.(*Config)

	if err := cp.Unmarshal(&c); err != nil {
		return err
	}

	receiversCfg, err := cp.Sub(receiversConfigKey)
	if err != nil {
		return fmt.Errorf("unable to extract key %v: %v", receiversConfigKey, err)
	}

	receiversSettings := cast.ToStringMap(cp.Get(receiversConfigKey))
	for subreceiverKey := range receiversSettings {
		subreceiverSection, err := receiversCfg.Sub(subreceiverKey)
		if err != nil {
			return fmt.Errorf("unable to extract subreceiver key %v: %v", subreceiverKey, err)
		}
		cfgSection := cast.ToStringMap(subreceiverSection.Get(configKey))
		subreceiver, err := newReceiverTemplate(subreceiverKey, cfgSection)
		if err != nil {
			return err
		}

		// Unmarshals receiver_creator configuration like rule.
		if err = subreceiverSection.Unmarshal(&subreceiver); err != nil {
			return fmt.Errorf("failed to deserialize sub-receiver %q: %s", subreceiverKey, err)
		}

		subreceiver.rule, err = newRule(subreceiver.Rule)
		if err != nil {
			return fmt.Errorf("subreceiver %q rule is invalid: %v", subreceiverKey, err)
		}

		c.receiverTemplates[subreceiverKey] = subreceiver
	}

	return nil
}
