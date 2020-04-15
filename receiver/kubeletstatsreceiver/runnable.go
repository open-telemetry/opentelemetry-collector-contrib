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

package kubeletstatsreceiver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
	// todo replace with scraping lib when it's ready
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/interval"
)

var _ interval.Runnable = (*runnable)(nil)

type runnable struct {
	ctx      context.Context
	cfg      *Config
	provider *kubelet.StatsProvider
	consumer consumer.MetricsConsumerOld
	logger   *zap.Logger
}

func newRunnable(
	ctx context.Context,
	consumer consumer.MetricsConsumerOld,
	cfg *Config,
	logger *zap.Logger,
) *runnable {
	return &runnable{
		ctx:      ctx,
		cfg:      cfg,
		consumer: consumer,
		logger:   logger,
	}
}

// Sets up the kubelet connection at startup time.
func (r *runnable) Setup() error {
	baseURL := ""
	if r.cfg.Endpoint != "" {
		baseURL = "https://" + r.cfg.Endpoint
	}
	// kubelet client will use the hostname if you pass in an empty url
	client, err := kubelet.NewClient(baseURL, &r.cfg.ClientConfig, r.logger)
	if err != nil {
		return err
	}
	rest := kubelet.NewRestClient(client)
	r.provider = kubelet.NewStatsProvider(rest)
	return nil
}

func (r *runnable) Run() error {
	const dataformat = "kubelet"
	const transport = "http"
	ctx := obsreport.StartMetricsReceiveOp(r.ctx, dataformat, transport)
	summary, err := r.provider.StatsSummary()
	if err != nil {
		obsreport.EndMetricsReceiveOp(ctx, dataformat, 0, 0, err)
		return nil
	}
	mds := kubelet.MetricsData(summary, typeStr)
	for _, md := range mds {
		err = r.consumer.ConsumeMetricsData(r.ctx, *md)
		numTimeSeries, numPoints := obsreport.CountMetricPoints(*md)
		obsreport.EndMetricsReceiveOp(ctx, dataformat, numTimeSeries, numPoints, err)
	}
	return nil
}
