// Copyright  OpenTelemetry Authors
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

package mongodbatlasreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const (
	typeStr            = "mongodbatlas"
	defaultGranularity = "PT1M" // 1-minute, as per https://docs.atlas.mongodb.com/reference/api/process-measurements/
)

// NewFactory creates a factory for MongoDB Atlas receiver
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	cfg := rConf.(*Config)
	ms, err := newMongoDBAtlasReceiver(ctx, params.Logger, cfg, consumer)
	if err != nil {
		return nil, fmt.Errorf("unable to create a MongoDB Atlas Receiver instance: %w", err)
	}
	return ms, err
}

func createDefaultConfig() config.Receiver {
	return &Config{
		Granularity:      defaultGranularity,
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
	}
}
