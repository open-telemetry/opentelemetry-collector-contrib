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

package jaegerlegacyreceiver

// This file implements factory for Jaeger receiver.

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/spf13/viper"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configprotocol"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr = "jaeger_legacy"

	// Protocol values.
	protoThriftTChannel = "thrift_tchannel"

	// Default endpoints to bind to.
	defaultTChannelBindEndpoint = "localhost:14267"
)

// Factory is the factory for Jaeger legacy receiver.
type Factory struct {
}

// Type gets the type of the Receiver config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

// CustomUnmarshaler is used to add defaults for named but empty protocols
func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return func(sourceViperSection *viper.Viper, intoCfg interface{}) error {
		if sourceViperSection == nil || len(sourceViperSection.AllKeys()) == 0 {
			return fmt.Errorf("jaeger legacy receiver config is empty")
		}

		// first load the config normally
		err := sourceViperSection.UnmarshalExact(intoCfg)
		if err != nil {
			return err
		}

		receiverCfg, ok := intoCfg.(*Config)
		if !ok {
			return fmt.Errorf("config type not *jaegerlegacyreceiver.Config")
		}

		// next manually search for protocols in viper that do not appear in the normally loaded config
		// these protocols were excluded during normal loading and we need to add defaults for them
		protocols := sourceViperSection.GetStringMap(protocolsFieldName)
		if len(protocols) == 0 {
			return fmt.Errorf("must specify at least one protocol when using the Jaeger Legacy receiver")
		}
		for k := range protocols {
			if _, ok := receiverCfg.Protocols[k]; !ok {
				if receiverCfg.Protocols[k], err = defaultsForProtocol(k); err != nil {
					return err
				}
			}
		}

		return nil
	}
}

// CreateDefaultConfig creates the default configuration for JaegerLegacy receiver.
func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		TypeVal:   configmodels.Type(typeStr),
		NameVal:   typeStr,
		Protocols: map[string]*configprotocol.ProtocolServerSettings{},
	}
}

// CreateTraceReceiver creates a trace receiver based on provided config.
func (f *Factory) CreateTraceReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	nextConsumer consumer.TraceConsumerOld,
) (component.TraceReceiver, error) {

	// Convert settings in the source config to Configuration struct
	// that JaegerLegacy receiver understands.

	rCfg := cfg.(*Config)

	protoTChannel := rCfg.Protocols[protoThriftTChannel]

	config := Configuration{}

	if protoTChannel != nil {
		var err error
		config.CollectorThriftPort, err = extractPortFromEndpoint(protoTChannel.Endpoint)
		if err != nil {
			return nil, err
		}
	}

	if protoTChannel == nil || config.CollectorThriftPort == 0 {
		err := fmt.Errorf("%v protocol endpoint with non-zero port must be enabled for %s receiver",
			protoThriftTChannel,
			configmodels.Type(typeStr),
		)
		return nil, err
	}

	// Create the receiver.
	return New(rCfg.Name(), &config, nextConsumer, logger)
}

// CreateMetricsReceiver creates a metrics receiver based on provided config.
func (f *Factory) CreateMetricsReceiver(
	logger *zap.Logger,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

// extract the port number from string in "address:port" format. If the
// port number cannot be extracted returns an error.
func extractPortFromEndpoint(endpoint string) (int, error) {
	_, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return 0, fmt.Errorf("endpoint is not formatted correctly: %s", err.Error())
	}
	port, err := strconv.ParseInt(portStr, 10, 0)
	if err != nil {
		return 0, fmt.Errorf("endpoint port is not a number: %s", err.Error())
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port number must be between 1 and 65535")
	}
	return int(port), nil
}

// returns a default value for a protocol name.  this really just boils down to the endpoint
func defaultsForProtocol(proto string) (*configprotocol.ProtocolServerSettings, error) {
	var defaultEndpoint string

	switch proto {
	case protoThriftTChannel:
		defaultEndpoint = defaultTChannelBindEndpoint
	default:
		return nil, fmt.Errorf("unknown Jaeger Legacy protocol %s", proto)
	}

	return &configprotocol.ProtocolServerSettings{
		Endpoint: defaultEndpoint,
	}, nil
}
