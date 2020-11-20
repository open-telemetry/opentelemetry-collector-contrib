// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package windowsperfcountersreceiver

import (
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

// This file implements Factory for WindowsPerfCounters receiver.

// The value of "type" key in configuration.
const typeStr = "windowsperfcounters"

// NewFactory creates a new factory for windows perf counters receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver),
		receiverhelper.WithCustomUnmarshaler(customUnmarshaler))
}

// customUnmarshaler returns custom unmarshaler for this config.
func customUnmarshaler(componentViperSection *viper.Viper, intoCfg interface{}) error {
	err := componentViperSection.Unmarshal(intoCfg)
	if err != nil {
		return err
	}

	return intoCfg.(*Config).validate()
}

// createDefaultConfig creates the default configuration for receiver.
func createDefaultConfig() configmodels.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: typeStr,
				NameVal: typeStr,
			},
			CollectionInterval: time.Minute,
		},
	}
}
