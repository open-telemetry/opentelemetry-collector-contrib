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

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

const (
	// The value of "type" key in configuration.
	typeStr                       = "icinga"
	defaultHost                   = "localhost:5665"
	defaultUsername               = "root"
	defaultPassword               = ""
	defaultDisableSslVerification = false
)

// NewFactory creates a factory for the StatsD receiver.
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver),
	)
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings:       config.NewReceiverSettings(config.NewComponentID(typeStr)),
		Host:                   defaultHost,
		Username:               defaultUsername,
		Password:               defaultPassword,
		DisableSslVerification: defaultDisableSslVerification,
		Histograms:             []HistogramConfig{},
	}
}

func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c := cfg.(*Config)
	err := c.Validate()
	if err != nil {
		return nil, err
	}
	return newIcingaReceiver(ctx, params, *c, consumer)
}
