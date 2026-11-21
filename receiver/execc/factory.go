// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package execc // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/execc"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var typeID = component.MustNewType("execc")

// NewFactory creates the execc receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeID,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Schedule: ScheduleConfig{
			Interval: time.Minute,
		},
	}
}

func createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	c := cfg.(*Config)
	return newExeccReceiver(c, next, set.Logger), nil
}
