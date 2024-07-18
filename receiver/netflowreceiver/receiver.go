// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"context"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type netflowReceiver struct {
	host        component.Host
	cancel      context.CancelFunc
	config      *Config
	logConsumer consumer.Logs
	logger      *zap.Logger
	listeners   []*Listener
}

func (nr *netflowReceiver) Start(ctx context.Context, host component.Host) error {
	return fmt.Errorf("not implemented")
}

func (nr *netflowReceiver) Shutdown(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}
