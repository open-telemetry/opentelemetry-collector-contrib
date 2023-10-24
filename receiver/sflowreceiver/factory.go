// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License. language governing permissions and
// limitations under the License.

package sflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sflowreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	stability = component.StabilityLevelAlpha
	typeStr   = "sflow"
)

func createDefaultConfig() component.Config {
	return &Config{
		confignet.NetAddr{
			Endpoint: "0.0.0.0:9995",
		},
		map[string]string{},
	}
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, stability),
	)
}

func createLogsReceiver(ctx context.Context, params receiver.CreateSettings, basecfg component.Config, nextConsumer consumer.Logs) (receiver.Logs, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	receiverConfig := basecfg.(*Config)

	receiver := &sflowreceiverlogs{
		createSettings: params,
		config:         receiverConfig,
		nextConsumer:   nextConsumer,
	}

	return receiver, nil
}
