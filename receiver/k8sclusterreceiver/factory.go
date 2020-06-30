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

package k8sclusterreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
)

const (
	// Value of "type" key in configuration.
	typeStr = "k8s_cluster"

	// Default config values.
	defaultCollectionInterval = 10 * time.Second
)

var defaultNodeConditionsToReport = []string{"Ready"}

var _ component.ReceiverFactoryOld = (*Factory)(nil)

// Factory is the factory for kubernetes-cluster receiver.
type Factory struct {
}

func (f Factory) Type() configmodels.Type {
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
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}
}

func (f Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

func (f Factory) CreateTraceReceiver(ctx context.Context, logger *zap.Logger, cfg configmodels.Receiver,
	consumer consumer.TraceConsumerOld) (component.TraceReceiver, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

func (f Factory) CreateMetricsReceiver(ctx context.Context, logger *zap.Logger, cfg configmodels.Receiver,
	consumer consumer.MetricsConsumerOld) (component.MetricsReceiver, error) {
	rCfg := cfg.(*Config)
	return newReceiver(logger, rCfg, consumer)
}
