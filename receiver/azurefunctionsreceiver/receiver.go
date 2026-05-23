// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

type functionsReceiver struct{}

func newFunctionsReceiver(_ *Config, _ receiver.Settings, _ consumer.Logs) receiver.Logs {
	return &functionsReceiver{}
}

func (*functionsReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*functionsReceiver) Shutdown(_ context.Context) error {
	return nil
}
