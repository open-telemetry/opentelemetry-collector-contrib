// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/udppoller"
)

const defaultEndpoint = "localhost:2000"

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
		AddrConfig: confignet.AddrConfig{
			Endpoint:  defaultEndpoint,
			Transport: udppoller.Transport,
		},
		ProxyServer: proxy.DefaultConfig(),
	}
}

func createTracesReceiver(ctx context.Context, params receiver.Settings, cfg component.Config, consumer consumer.Traces) (receiver.Traces, error) {
	rcfg, ok := cfg.(*Config)
	if !ok {
		params.Logger.Error("Failed to cast receiver config")
		return nil, fmt.Errorf("invalid config type")
	}

	if rcfg.Region == "" {
		rcfg.Region = "us-west-2"
	}

	client, err := newXRayClient(ctx, rcfg.Region)
	if err != nil {
		params.Logger.Error("Failed to create AWS X-Ray client", zap.Error(err))
		return nil, err
	}

	return newReceiver(rcfg, consumer, params, client)
}
