// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type netflowReceiver struct {
	// host        component.Host
	// cancel      context.CancelFunc
	config      *Config
	logConsumer consumer.Logs
	logger      *zap.Logger
	// listeners   []*Listener
}

func (nr *netflowReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (nr *netflowReceiver) Shutdown(_ context.Context) error {
	return nil
}
