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

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
	// todo replace with scraping lib when it's ready
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/interval"
)

var _ interval.Runnable = (*runnable)(nil)

type runnable struct {
	ctx        context.Context
	provider   *kubelet.StatsProvider
	consumer   consumer.MetricsConsumerOld
	logger     *zap.Logger
	restClient kubelet.RestClient
}

func newRunnable(
	ctx context.Context,
	consumer consumer.MetricsConsumerOld,
	restClient kubelet.RestClient,
	logger *zap.Logger,
) *runnable {
	return &runnable{
		ctx:        ctx,
		consumer:   consumer,
		restClient: restClient,
		logger:     logger,
	}
}

// Sets up the kubelet connection at startup time.
func (r *runnable) Setup() error {
	r.provider = kubelet.NewStatsProvider(r.restClient)
	return nil
}

func (r *runnable) Run() error {
	const dataformat = "kubelet"
	const transport = "http"
	ctx := obsreport.StartMetricsReceiveOp(r.ctx, dataformat, transport)
	summary, err := r.provider.StatsSummary()
	if err != nil {
		r.logger.Error("StatsSummary failed", zap.Error(err))
		obsreport.EndMetricsReceiveOp(ctx, dataformat, 0, 0, err)
		return nil
	}
	mds := kubelet.MetricsData(summary, typeStr)
	for _, md := range mds {
		err = r.consumer.ConsumeMetricsData(r.ctx, *md)
		if err != nil {
			r.logger.Error("ConsumeMetricsData failed", zap.Error(err))
		}
		numTimeSeries, numPoints := obsreport.CountMetricPoints(*md)
		obsreport.EndMetricsReceiveOp(ctx, dataformat, numTimeSeries, numPoints, err)
	}
	return nil
}
