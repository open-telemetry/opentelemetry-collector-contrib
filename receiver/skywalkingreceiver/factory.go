// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

// This file implements factory for skywalking receiver.

import (
	"context"
	"fmt"
	"go.opentelemetry.io/collector/config/confighttp"
	"net"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const (
	typeStr = "skywalking"

	// Protocol values.
	protoGRPC = "grpc"

	// Default endpoints to bind to.
	defaultGRPCBindEndpoint = "0.0.0.0:11800"
	defaultHTTPBindEndpoint = "0.0.0.0:12800"
)

// NewFactory creates a new Jaeger receiver factory.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithTraces(createTracesReceiver))
}

// CreateDefaultConfig creates the default configuration for Skywalking receiver.
func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		Protocols: Protocols{
			GRPC: &configgrpc.GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint:  defaultGRPCBindEndpoint,
					Transport: "tcp",
				},
			},
			HTTP: &confighttp.HTTPServerSettings{
				Endpoint: defaultHTTPBindEndpoint,
			},
		},
	}
}

// createTracesReceiver creates a trace receiver based on provided config.
func createTracesReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Traces,
) (component.TracesReceiver, error) {

	// Convert settings in the source config to configuration struct
	// that Skywalking receiver understands.
	rCfg := cfg.(*Config)

	var config configuration
	// Set ports
	if rCfg.Protocols.GRPC != nil {
		config.CollectorGRPCServerSettings = *rCfg.Protocols.GRPC
		config.CollectorGRPCPort, _ = extractPortFromEndpoint(rCfg.Protocols.GRPC.NetAddr.Endpoint)
	}

	if rCfg.Protocols.HTTP != nil {
		config.CollectorHTTPSettings = *rCfg.Protocols.HTTP
		config.CollectorHTTPPort, _ = extractPortFromEndpoint(rCfg.Protocols.HTTP.Endpoint)
	}

	// Create the receiver.
	return newSkywalkingReceiver(rCfg.ID(), &config, nextConsumer, set), nil
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
