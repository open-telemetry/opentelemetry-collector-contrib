// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		HuaweiSessionConfig: HuaweiSessionConfig{
			NoVerifySSL: false,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	next consumer.Metrics) (receiver.Metrics, error) {

	cesCfg := cfg.(*Config)

	cesReceiver := newHuaweiCloudCesReceiver(params, cesCfg, next)

	return cesReceiver, nil

}
