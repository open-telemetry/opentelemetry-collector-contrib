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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

// LogReceiverType is the interface used by stanza-based log receivers
type LogReceiverType interface {
	Type() component.Type
	CreateDefaultConfig() component.ReceiverConfig
	BaseConfig(component.ReceiverConfig) BaseConfig
	InputConfig(component.ReceiverConfig) operator.Config
}

// NewFactory creates a factory for a Stanza-based receiver
func NewFactory(logReceiverType LogReceiverType, sl component.StabilityLevel) component.ReceiverFactory {
	return component.NewReceiverFactory(
		logReceiverType.Type(),
		logReceiverType.CreateDefaultConfig,
		component.WithLogsReceiver(createLogsReceiver(logReceiverType), sl),
	)
}

func createLogsReceiver(logReceiverType LogReceiverType) component.CreateLogsReceiverFunc {
	return func(
		ctx context.Context,
		params component.ReceiverCreateSettings,
		cfg component.ReceiverConfig,
		nextConsumer consumer.Logs,
	) (component.LogsReceiver, error) {
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
		obsrecv := obsreport.MustNewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             cfg.ID(),
			ReceiverCreateSettings: params,
		})
		return &receiver{
			id:        cfg.ID(),
			pipe:      pipe,
			emitter:   emitter,
			consumer:  nextConsumer,
			logger:    params.Logger,
			converter: converter,
			obsrecv:   obsrecv,
			storageID: baseCfg.StorageID,
		}, nil
	}
}
