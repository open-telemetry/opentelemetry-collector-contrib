// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachedruidreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachedruidreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

type metricsReceiver struct {
}

func newMetricsReceiver(config *Config, settings receiver.CreateSettings, nextConsumer consumer.Metrics) (*metricsReceiver, error) {
	return &metricsReceiver{}, nil
}

func (r *metricsReceiver) Start(_ context.Context, host component.Host) error {
	return nil
}

func (r *metricsReceiver) Shutdown(_ context.Context) error {
	return nil
}
