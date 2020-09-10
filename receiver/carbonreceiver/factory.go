// Copyright 2019, OpenTelemetry Authors
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

package carbonreceiver

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport"
)

// This file implements factory for Carbon receiver.

const (
	// The value of "type" key in configuration.
	typeStr = "carbon"
)

// NewFactory creates a factory for Carbon receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithCustomUnmarshaler(customUnmarshaler),
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func customUnmarshaler(sourceViperSection *viper.Viper, intoCfg interface{}) error {
	if sourceViperSection == nil {
		// The section is empty nothing to do, using the default config.
		return nil
	}

	// Unmarshal but not exact yet so the different keys under config do not
	// trigger errors, this is needed so that the types of protocol and transport
	// are read.
	if err := sourceViperSection.Unmarshal(intoCfg); err != nil {
		return err
	}

	// Unmarshal the protocol, so the type of config can be properly set.
	rCfg := intoCfg.(*Config)
	vParserCfg := sourceViperSection.Sub(parserConfigSection)
	if vParserCfg != nil {
		if err := protocol.LoadParserConfig(vParserCfg, rCfg.Parser); err != nil {
			return fmt.Errorf(
				"error on %q section for %s: %v",
				parserConfigSection,
				rCfg.Name(),
				err)
		}
	}

	// Unmarshal exact to validate the config keys.
	if err := sourceViperSection.UnmarshalExact(intoCfg); err != nil {
		return err
	}

	return nil
}

func createDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		NetAddr: confignet.NetAddr{
			Endpoint:  "localhost:2003",
			Transport: "tcp",
		},
		TCPIdleTimeout: transport.TCPIdleTimeoutDefault,
		Parser: &protocol.Config{
			Type:   "plaintext",
			Config: &protocol.PlaintextConfig{},
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {

	rCfg := cfg.(*Config)
	return New(params.Logger, *rCfg, consumer)
}
