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
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport"
)

// This file implements factory for Carbon receiver.

const (
	// The value of "type" key in configuration.
	typeStr = "carbon"
)

// Factory is the factory for carbon receiver.
type Factory struct {
}

var _ component.ReceiverFactoryOld = (*Factory)(nil)

// Type gets the type of the Receiver config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

// CustomUnmarshaler returns the custom function to handle the special settings
// used on the receiver.
func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return func(sourceViperSection *viper.Viper, intoCfg interface{}) error {
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
}

// CreateDefaultConfig creates the default configuration for Carbon receiver.
func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		Endpoint:       "localhost:2003",
		Transport:      "tcp",
		TCPIdleTimeout: transport.TCPIdleTimeoutDefault,
		Parser: &protocol.Config{
			Type:   "plaintext",
			Config: &protocol.PlaintextConfig{},
		},
	}
}

// CreateTraceReceiver creates a trace receiver based on provided config.
func (f *Factory) CreateTraceReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	consumer consumer.TraceConsumerOld,
) (component.TraceReceiver, error) {

	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsReceiver creates a metrics receiver based on provided config.
func (f *Factory) CreateMetricsReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {

	rCfg := cfg.(*Config)
	return New(logger, *rCfg, consumer)
}
