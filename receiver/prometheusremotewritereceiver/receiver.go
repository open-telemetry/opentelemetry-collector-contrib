// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

func newRemoteWriteReceiver(_ receiver.Settings, _ *Config, _ consumer.Metrics) (receiver.Metrics, error) {
	panic("need implement")
}

// nolint
type prometheusRemoteWriteReceiver struct {
	settings     receiver.Settings //nolint:unused
	host         component.Host    //nolint:unused
	nextConsumer consumer.Metrics  //nolint:unused

	config *Config     //nolint:unused
	logger *zap.Logger //nolint:unused
}

// nolint
func (prwc *prometheusRemoteWriteReceiver) Start(_ context.Context, _ component.Host) error {
	panic("need implement")
}
