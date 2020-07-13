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

package elasticsearchreceiver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/query"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/stats"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
)

const (
	queryTypeStr = "elasticsearch-query"
	statsTypeStr = "elasticsearch-stats"
)

var _ component.MetricsReceiverFactory = (*QueryFactory)(nil)

// QueryFactory creates a RedisReceiver.
type QueryFactory struct {
}

func (f *QueryFactory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

// Type returns the type of this factory, "redis".
func (f *QueryFactory) Type() configmodels.Type {
	return queryTypeStr
}

// CreateDefaultConfig creates a default config.
func (f *QueryFactory) CreateDefaultConfig() configmodels.Receiver {
	return &query.DefaultConfig
}

func (f *QueryFactory) CreateMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	return query.NewESReceiver(params.Logger, cfg.(*query.Config), consumer), nil
}

var _ component.MetricsReceiverFactory = (*StatsFactory)(nil)

// StatsFactory creates a RedisReceiver.
type StatsFactory struct {
}

func (f *StatsFactory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

// Type returns the type of this factory, "redis".
func (f *StatsFactory) Type() configmodels.Type {
	return statsTypeStr
}

// CreateDefaultConfig creates a default config.
func (f *StatsFactory) CreateDefaultConfig() configmodels.Receiver {
	return &stats.DefaultConfig
}

func (f *StatsFactory) CreateMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	return stats.NewESReceiver(params.Logger, cfg.(*stats.Config), consumer), nil
}
