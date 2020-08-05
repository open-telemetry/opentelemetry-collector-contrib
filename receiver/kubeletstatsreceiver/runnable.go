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
	v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
	// todo replace with scraping lib when it's ready
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/interval"
)

var _ interval.Runnable = (*runnable)(nil)

type runnable struct {
	ctx                 context.Context
	receiverName        string
	statsProvider       *kubelet.StatsProvider
	metadataProvider    *kubelet.MetadataProvider
	consumer            consumer.MetricsConsumerOld
	logger              *zap.Logger
	restClient          kubelet.RestClient
	extraMetadataLabels []kubelet.MetadataLabel
}

func newRunnable(
	ctx context.Context,
	receiverName string,
	consumer consumer.MetricsConsumerOld,
	restClient kubelet.RestClient,
	extraMetadataLabels []kubelet.MetadataLabel,
	logger *zap.Logger,
) *runnable {
	return &runnable{
		ctx:                 ctx,
		receiverName:        receiverName,
		consumer:            consumer,
		restClient:          restClient,
		logger:              logger,
		extraMetadataLabels: extraMetadataLabels,
	}
}

// Sets up the kubelet connection at startup time.
func (r *runnable) Setup() error {
	r.statsProvider = kubelet.NewStatsProvider(r.restClient)
	r.metadataProvider = kubelet.NewMetadataProvider(r.restClient)
	return nil
}

func (r *runnable) Run() error {
	const transport = "http"
	summary, err := r.statsProvider.StatsSummary()
	if err != nil {
		r.logger.Error("call to /stats/summary endpoint failed", zap.Error(err))
		return nil
	}

	var podsMetadata *v1.PodList
	// fetch metadata only when extra metadata labels are needed
	if len(r.extraMetadataLabels) > 0 {
		podsMetadata, err = r.metadataProvider.Pods()
		if err != nil {
			r.logger.Error("call to /pods endpoint failed", zap.Error(err))
			return nil
		}
	}

	metadata := kubelet.NewMetadata(r.extraMetadataLabels, podsMetadata)
	mds := kubelet.MetricsData(r.logger, summary, metadata, typeStr)
	ctx := obsreport.ReceiverContext(r.ctx, typeStr, transport, r.receiverName)
	for _, md := range mds {
		ctx = obsreport.StartMetricsReceiveOp(ctx, typeStr, transport)
		err = r.consumer.ConsumeMetricsData(ctx, *md)
		var numTimeSeries, numPoints int
		if err != nil {
			r.logger.Error("ConsumeMetricsData failed", zap.Error(err))
		} else {
			numTimeSeries, numPoints = obsreport.CountMetricPoints(*md)
		}
		obsreport.EndMetricsReceiveOp(ctx, typeStr, numTimeSeries, numPoints, err)
	}
	return nil
}
