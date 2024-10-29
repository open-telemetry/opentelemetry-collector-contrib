// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudlogsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudlogsreceiver"

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/huawei"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudlogsreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
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
		HuaweiSessionConfig: huawei.HuaweiSessionConfig{
			NoVerifySSL: false,
		},
	}
}

func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	next consumer.Logs) (receiver.Logs, error) {
	return newHuaweiCloudLogsReceiver(params, cfg.(*Config), next), nil

}
