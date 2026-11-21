// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpjson // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpjson"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var typeID = component.MustNewType("httpjson")

// NewFactory creates the httpjson receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeID,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Method:   "GET",
		Interval: time.Minute,
	}
}

func createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	c := cfg.(*Config)
	return newHTTPJSONReceiver(c, next, set.Logger), nil
}
