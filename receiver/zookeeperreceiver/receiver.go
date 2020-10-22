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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

type zookeeperMetricsReceiver struct {
	scraper receiverhelper.MetricsScraper
}

var _ component.MetricsReceiver = (*zookeeperMetricsReceiver)(nil)

func (z *zookeeperMetricsReceiver) Start(ctx context.Context, _ component.Host) error {
	z.scraper.Initialize(ctx)
	return nil
}

func (z *zookeeperMetricsReceiver) Shutdown(ctx context.Context) error {
	z.scraper.Close(ctx)
	return nil
}

func (z *zookeeperMetricsReceiver) scrape(_ context.Context) (pdata.MetricSlice, error) {
	return pdata.MetricSlice{}, nil
}
