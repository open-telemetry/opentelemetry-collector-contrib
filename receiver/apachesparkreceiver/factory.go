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

package apachesparkreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/metadata"
)

var errConfigNotSpark = errors.New("config was not a Spark receiver config")

type SparkReceiver struct {
	component.StartFunc
	component.ShutdownFunc
}

// NewFactory creates a new receiver factory for Spark
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

// createDefaultConfig creates a config for Spark with as many default values as possible
func createDefaultConfig() component.Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: defaultCollectionInterval,
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: defaultEndpoint,
		},
	}
}

// createMetricsReceiver creates the metric receiver for Spark
func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	_, ok := config.(*Config)
	if !ok {
		return nil, errConfigNotSpark
	}

	return SparkReceiver{}, nil
}
