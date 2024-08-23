// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

// LogReceiverType is the interface used by stanza-based log receivers
type LogReceiverType interface {
	Type() component.Type
	CreateDefaultConfig() component.Config
	BaseConfig(component.Config) BaseConfig
	InputConfig(component.Config) operator.Config
}

// NewFactory creates a factory for a Stanza-based receiver
func NewFactory(logReceiverType LogReceiverType, sl component.StabilityLevel) rcvr.Factory {
	return rcvr.NewFactory(
		logReceiverType.Type(),
		logReceiverType.CreateDefaultConfig,
		rcvr.WithLogs(createLogsReceiver(logReceiverType), sl),
	)
}

func createLogsReceiver(logReceiverType LogReceiverType) rcvr.CreateLogsFunc {
	return func(
		_ context.Context,
		params rcvr.Settings,
		cfg component.Config,
		nextConsumer consumer.Logs,
	) (rcvr.Logs, error) {
		inputCfg := logReceiverType.InputConfig(cfg)
		baseCfg := logReceiverType.BaseConfig(cfg)

		operators := append([]operator.Config{inputCfg}, baseCfg.Operators...)

		var emitterOpts []helper.EmitterOption
		if baseCfg.maxBatchSize > 0 {
			emitterOpts = append(emitterOpts, helper.WithMaxBatchSize(baseCfg.maxBatchSize))
		}
		if baseCfg.flushInterval > 0 {
			emitterOpts = append(emitterOpts, helper.WithFlushInterval(baseCfg.flushInterval))
		}
		emitter := helper.NewLogEmitter(params.TelemetrySettings, emitterOpts...)
		pipe, err := pipeline.Config{
			Operators:     operators,
			DefaultOutput: emitter,
		}.Build(params.TelemetrySettings)
		if err != nil {
			return nil, err
		}

		var converterOpts []converterOption
		if baseCfg.numWorkers > 0 {
			converterOpts = append(converterOpts, withWorkerCount(baseCfg.numWorkers))
		}
		converter := NewConverter(params.TelemetrySettings, converterOpts...)
		obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
			ReceiverID:             params.ID,
			ReceiverCreateSettings: params,
		})
		if err != nil {
			return nil, err
		}
		return &receiver{
			set:       params.TelemetrySettings,
			id:        params.ID,
			pipe:      pipe,
			emitter:   emitter,
			consumer:  consumerretry.NewLogs(baseCfg.RetryOnFailure, params.Logger, nextConsumer),
			converter: converter,
			obsrecv:   obsrecv,
			storageID: baseCfg.StorageID,
		}, nil
	}
}
