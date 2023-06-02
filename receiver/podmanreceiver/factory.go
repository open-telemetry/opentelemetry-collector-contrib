// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	typeStr           = "podman_stats"
	stability         = component.StabilityLevelDevelopment
	defaultAPIVersion = "3.3.1"
)

func NewFactory() rcvr.Factory {
	return rcvr.NewFactory(
		typeStr,
		createDefaultReceiverConfig,
		rcvr.WithMetrics(createMetricsReceiver, stability))
}

func createDefaultConfig() *Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
		},
		Endpoint:   "unix:///run/podman/podman.sock",
		Timeout:    5 * time.Second,
		APIVersion: defaultAPIVersion,
	}
}

func createDefaultReceiverConfig() component.Config {
	return createDefaultConfig()
}

func createMetricsReceiver(
	ctx context.Context,
	params rcvr.CreateSettings,
	config component.Config,
	consumer consumer.Metrics,
) (rcvr.Metrics, error) {
	podmanConfig := config.(*Config)
	dsr, err := newReceiver(ctx, params, podmanConfig, consumer, nil)
	if err != nil {
		return nil, err
	}

	return dsr, nil
}
