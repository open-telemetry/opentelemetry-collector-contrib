// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver"

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
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
		BackOffConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     100 * time.Millisecond,
			MaxInterval:         time.Second,
			MaxElapsedTime:      15 * time.Second,
			RandomizationFactor: backoff.DefaultRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
		},
		huaweiSessionConfig: huaweiSessionConfig{
			NoVerifySSL: false,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	return newHuaweiCloudCesReceiver(params, cfg.(*Config), next), nil
}
