// Copyright 2020, OpenTelemetry Authors
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

package kubernetesclusterreceiver

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configerror"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"go.uber.org/zap"
)

const (
	// Value of "type" key in configuration.
	typeStr = "k8s_cluster"

	// Default config values.
	defaultCollectionInterval = 10 * time.Second
)

var defaultNodeConditionsToReport = []string{"Ready"}

var _ component.ReceiverFactoryOld = &Factory{}

// Factory is the factory for kubernetes-cluster receiver.
type Factory struct {
}

func (f Factory) Type() string {
	return typeStr
}

func (f Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		CollectionInterval:         defaultCollectionInterval,
		NodeConditionTypesToReport: defaultNodeConditionsToReport,
	}
}

func (f Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

func (f Factory) CreateTraceReceiver(ctx context.Context, logger *zap.Logger, cfg configmodels.Receiver,
	consumer consumer.TraceConsumerOld) (component.TraceReceiver, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

func (f Factory) CreateMetricsReceiver(logger *zap.Logger, cfg configmodels.Receiver,
	consumer consumer.MetricsConsumerOld) (component.MetricsReceiver, error) {
	rCfg := cfg.(*Config)
	return newReceiver(logger, rCfg, consumer)
}
