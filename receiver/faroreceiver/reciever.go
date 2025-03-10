// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
)

type faroReceiver struct{}

func newTraceReceiverTraces(_ *Config, _ consumer.Traces) *faroReceiver {
	return &faroReceiver{}
}

func newTraceReceiverLogs(_ *Config, _ consumer.Logs) *faroReceiver {
	return &faroReceiver{}
}

func (r *faroReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (r *faroReceiver) Shutdown(_ context.Context) error {
	return nil
}
