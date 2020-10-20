// Copyright The OpenTelemetry Authors
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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/udppoller"
)

// NewFactory creates a factory for AWS receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		awsxray.TypeStr,
		createDefaultConfig,
		receiverhelper.WithTraces(createTraceReceiver))
}

func createDefaultConfig() configmodels.Receiver {
	// reference the existing default configurations provided
	// in the X-Ray daemon:
	// https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L99
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type(awsxray.TypeStr),
			NameVal: awsxray.TypeStr,
			// X-Ray daemon defaults to 127.0.0.1:2000 but
			// the default in OT is 0.0.0.0.
		},
		NetAddr: confignet.NetAddr{
			Endpoint:  "0.0.0.0:2000",
			Transport: udppoller.Transport,
		},
		ProxyServer: proxy.DefaultConfig(),
	}
}

func createTraceReceiver(
	ctx context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.TracesConsumer) (component.TraceReceiver, error) {
	rcfg := cfg.(*Config)
	return newReceiver(rcfg, consumer, params.Logger)
}
