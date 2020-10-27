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

package zookeeperreceiver

import (
	"context"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type zookeeperMetricsScraper struct {
}

func newZookeeperMetricsScraper(logger *zap.Logger, config *Config) (*zookeeperMetricsScraper, error) {
	return &zookeeperMetricsScraper{}, nil
}

func (z *zookeeperMetricsScraper) Initialize(_ context.Context) error {
	return nil
}

func (z *zookeeperMetricsScraper) Close(_ context.Context) error {
	return nil
}

func (z *zookeeperMetricsScraper) Scrape(_ context.Context) (pdata.ResourceMetricsSlice, error) {
	return pdata.ResourceMetricsSlice{}, nil
}
