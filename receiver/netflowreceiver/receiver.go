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
	config      *Config
	logConsumer consumer.Logs
	logger      *zap.Logger
	listener    *Listener
}

func (nr *netflowReceiver) Start(ctx context.Context, host component.Host) error {
	// TODO - Pass ctx and host here
	listener := newListener(*nr.config, nr.logger, nr.logConsumer)
	if err := listener.Start(); err != nil {
		return err
	}
	nr.listener = listener
	nr.logger.Info("NetFlow receiver started")
	return nil
}

func (nr *netflowReceiver) Shutdown(_ context.Context) error {
	nr.logger.Info("NetFlow receiver is shutting down")
	err := nr.listener.Shutdown()
	if err != nil {
		return err
	}
	return nil
}
