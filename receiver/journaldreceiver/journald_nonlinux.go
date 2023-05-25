// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux
// +build !linux

package journaldreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver/internal/metadata"
)

// NewFactory creates a dummy factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

type JournaldConfig struct {
	adapter.BaseConfig `mapstructure:",squash"`
}

func createDefaultConfig() component.Config {
	return &JournaldConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
	}
}

func createLogsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	return nil, fmt.Errorf("journald is only supported on linux")
}
