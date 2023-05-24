// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstransformprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstransformprocessor/internal/metadata"
)

// NewFactory returns a new factory for the Logs Transform processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability))
}

// Note: This isn't a valid configuration because the processor would do no work.
func createDefaultConfig() component.Config {
	return &Config{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
	}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs) (processor.Logs, error) {
	pCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("could not initialize logs transform processor")
	}

	if len(pCfg.BaseConfig.Operators) == 0 {
		return nil, errors.New("no operators were configured for this logs transform processor")
	}

	return newProcessor(pCfg, nextConsumer, set.Logger)
}
