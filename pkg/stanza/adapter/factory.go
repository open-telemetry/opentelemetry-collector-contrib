// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	rcvr "go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
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
		ctx context.Context,
		params rcvr.CreateSettings,
		cfg component.Config,
		nextConsumer consumer.Logs,
	) (rcvr.Logs, error) {
		inputCfg := logReceiverType.InputConfig(cfg)
		baseCfg := logReceiverType.BaseConfig(cfg)

		operators := append([]operator.Config{inputCfg}, baseCfg.Operators...)

		emitter := NewLogEmitter(params.Logger.Sugar())
		pipe, err := pipeline.Config{
			Operators:     operators,
			DefaultOutput: emitter,
		}.Build(params.Logger.Sugar())
		if err != nil {
			return nil, err
		}

		converter := NewConverter(params.Logger)
		obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             params.ID,
			ReceiverCreateSettings: params,
		})
		if err != nil {
			return nil, err
		}
		return &receiver{
			id:        params.ID,
			pipe:      pipe,
			emitter:   emitter,
			consumer:  consumerretry.NewLogs(baseCfg.RetryOnFailure, params.Logger, nextConsumer),
			logger:    params.Logger,
			converter: converter,
			obsrecv:   obsrecv,
			storageID: baseCfg.StorageID,
		}, nil
	}
}
