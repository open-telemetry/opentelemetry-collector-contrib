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

package stdinreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// This file implements factory for stdin receiver.

const (
	// The value of "type" key in configuration.
	typeStr = "stdin"
)

type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
}

// NewFactory creates a factory for stdin receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithLogs(createLogsReceiver))
}

// CreateDefaultConfig creates the default configuration for stdin receiver.
func createDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
	}
}

// createLogsReceiver creates a logs receiver based on provided config.
func createLogsReceiver(
	_ context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.LogsConsumer,
) (component.LogsReceiver, error) {

	rCfg := cfg.(*Config)

	return NewLogsReceiver(params.Logger, *rCfg, consumer)
}
