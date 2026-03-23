// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscwotlpbatchsplitprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awscwotlpbatchsplitprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awscwotlpbatchsplitprocessor/internal/metadata"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		MaxRequestByteSize: defaultMaxRequestByteSize,
	}
}

func createLogsProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	pCfg := cfg.(*Config)
	return &awsCWOTLPBatchLogProcessor{
		logger:             set.Logger,
		nextConsumer:       nextConsumer,
		maxRequestByteSize: pCfg.MaxRequestByteSize,
		baseLogBufferSize:  defaultBaseLogBufferSize,
	}, nil
}
