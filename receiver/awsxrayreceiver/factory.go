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

package awsxrayreceiver

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
)

// ensure the Factory implements the ReceiverFactory interface
var _ component.ReceiverFactory = (*Factory)(nil)

// Factory is the factory for creating AWS X-Ray receiver instances.
type Factory struct {
}

// Type returns the type of the Receiver configuration created by this factory.
func (f *Factory) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

// CustomUnmarshaler returns nil because there's no need for custom unmarshaling.
func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

// CreateDefaultConfig returns the default configurations for a new AWS X-Ray receiver.
func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	// reference the existing default configurations provided
	// in the X-Ray daemon:
	// https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L99
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
			// X-Ray daemon defaults to 127.0.0.1:2000 but
			// the default in OT is 0.0.0.0.
		},
		TCPAddr: confignet.TCPAddr{
			Endpoint: "0.0.0.0:2000",
		},
		ProxyServer: &proxyServer{
			TCPEndpoint:  "0.0.0.0:2000",
			ProxyAddress: "",
			TLSSetting: configtls.TLSClientSetting{
				Insecure:   false,
				ServerName: "",
			},
			Region:      "",
			RoleARN:     "",
			AWSEndpoint: "",
			LocalMode:   aws.Bool(false),
		},
	}
}

// CreateTraceReceiver creates an AWS X-Ray receiver.
func (f *Factory) CreateTraceReceiver(
	ctx context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	nextConsumer consumer.TraceConsumer) (component.TraceReceiver, error) {
	// TODO: Finish the implementation
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsReceiver merely returns an error because the X-Ray receiver does not
// support ingesting metrics.
func (f *Factory) CreateMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}
