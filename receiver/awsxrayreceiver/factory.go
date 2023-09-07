// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/udppoller"
)

// NewFactory creates a factory for AWS receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability))
}

func createDefaultConfig() component.Config {
	// reference the existing default configurations provided
	// in the X-Ray daemon:
	// https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L99
	return &Config{
		// X-Ray daemon defaults to 127.0.0.1:2000 but
		// the default in OT is 0.0.0.0.
		NetAddr: confignet.NetAddr{
			Endpoint:  "0.0.0.0:2000",
			Transport: udppoller.Transport,
		},
		ProxyServer: proxy.DefaultConfig(),
	}
}

func createTracesReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Traces) (receiver.Traces, error) {
	rcfg := cfg.(*Config)
	return newReceiver(rcfg, consumer, params)
}
