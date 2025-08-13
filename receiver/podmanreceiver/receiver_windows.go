// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func newMetricsReceiver(
	_ receiver.Settings,
	_ *Config,
	_ consumer.Metrics,
	_ any,
) (receiver.Metrics, error) {
	return nil, errors.New("podman receiver is not supported on windows")
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	podmanConfig := config.(*Config)

	return newMetricsReceiver(params, podmanConfig, nil, consumer)
}
