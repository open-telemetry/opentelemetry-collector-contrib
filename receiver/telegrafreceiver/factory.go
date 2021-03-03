// Copyright 2021, OpenTelemetry Authors
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

package telegrafreceiver

import (
	"context"
	"fmt"

	telegrafagent "github.com/influxdata/telegraf/agent"
	telegrafconfig "github.com/influxdata/telegraf/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const (
	typeStr    = "telegraf"
	versionStr = "v0.1"
)

// NewFactory creates a factory for telegraf receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver),
	)
}

func createDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		SeparateField: false,
	}
}

// createMetricsReceiver creates a metrics receiver based on provided config.
func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	nextConsumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	tCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("failed reading telegraf agent config from otc config")
	}

	tConfig := telegrafconfig.NewConfig()
	if err := tConfig.LoadConfigData([]byte(tCfg.AgentConfig)); err != nil {
		return nil, fmt.Errorf("failed loading telegraf agent config: %w", err)
	}
	tAgent, err := telegrafagent.NewAgent(tConfig)
	if err != nil {
		return nil, fmt.Errorf("failed creating telegraf agent: %w", err)
	}

	return &telegrafreceiver{
		agent:           tAgent,
		consumer:        nextConsumer,
		logger:          params.Logger,
		metricConverter: newConverter(tCfg.SeparateField),
	}, nil
}
