// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

var synchronousLogEmitterFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"stanza.synchronousLogEmitter",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Prevents possible data loss in Stanza-based receivers by emitting logs synchronously."),
	featuregate.WithRegisterFromVersion("v0.122.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/35456"),
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

		obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
			ReceiverID:             params.ID,
			ReceiverCreateSettings: params,
		})
		if err != nil {
			return nil, err
		}
		rcv := &receiver{
			set:       params.TelemetrySettings,
			id:        params.ID,
			consumer:  consumerretry.NewLogs(baseCfg.RetryOnFailure, params.Logger, nextConsumer),
			obsrecv:   obsrecv,
			storageID: baseCfg.StorageID,
		}

		var emitterOpts []helper.EmitterOption
		if baseCfg.maxBatchSize > 0 {
			emitterOpts = append(emitterOpts, helper.WithMaxBatchSize(baseCfg.maxBatchSize))
		}
		if baseCfg.flushInterval > 0 {
			emitterOpts = append(emitterOpts, helper.WithFlushInterval(baseCfg.flushInterval))
		}

		var emitter helper.LogEmitter
		if synchronousLogEmitterFeatureGate.IsEnabled() {
			emitter = helper.NewSynchronousLogEmitter(params.TelemetrySettings, rcv.consumeEntries)
		} else {
			emitter = helper.NewBatchingLogEmitter(params.TelemetrySettings, rcv.consumeEntries, emitterOpts...)
		}

		pipe, err := pipeline.Config{
			Operators:     operators,
			DefaultOutput: emitter,
		}.Build(params.TelemetrySettings)
		if err != nil {
			return nil, err
		}

		rcv.emitter = emitter
		rcv.pipe = pipe

		return rcv, nil
	}
}
