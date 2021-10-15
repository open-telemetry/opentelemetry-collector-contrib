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
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	// todo replace with scraping lib when it's ready
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
)

var _ interval.Runnable = (*runnable)(nil)

const transport = "http"

type runnable struct {
	ctx                   context.Context
	statsProvider         *kubelet.StatsProvider
	metadataProvider      *kubelet.MetadataProvider
	consumer              consumer.Metrics
	logger                *zap.Logger
	restClient            kubelet.RestClient
	extraMetadataLabels   []kubelet.MetadataLabel
	metricGroupsToCollect map[kubelet.MetricGroup]bool
	k8sAPIClient          kubernetes.Interface
	cachedVolumeLabels    map[string]map[string]string
	obsrecv               *obsreport.Receiver
}

func newRunnable(
	ctx context.Context,
	consumer consumer.Metrics,
	restClient kubelet.RestClient,
	set component.ReceiverCreateSettings,
	rOptions *receiverOptions,
) *runnable {
	return &runnable{
		ctx:                   ctx,
		consumer:              consumer,
		restClient:            restClient,
		logger:                set.Logger,
		extraMetadataLabels:   rOptions.extraMetadataLabels,
		metricGroupsToCollect: rOptions.metricGroupsToCollect,
		k8sAPIClient:          rOptions.k8sAPIClient,
		cachedVolumeLabels:    make(map[string]map[string]string),
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             rOptions.id,
			Transport:              transport,
			ReceiverCreateSettings: set,
		}),
	}
}

// Setup the kubelet connection at startup time.
func (r *runnable) Setup() error {
	r.statsProvider = kubelet.NewStatsProvider(r.restClient)
	r.metadataProvider = kubelet.NewMetadataProvider(r.restClient)
	return nil
}

func (r *runnable) Run() error {
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

	metadata := kubelet.NewMetadata(r.extraMetadataLabels, podsMetadata, r.detailedPVCLabelsSetter())
	mds := kubelet.MetricsData(r.logger, summary, metadata, typeStr, r.metricGroupsToCollect)
	metrics := pdata.NewMetrics()
	for i := range mds {
		mds[i].ResourceMetrics().MoveAndAppendTo(metrics.ResourceMetrics())
	}

	ctx := r.obsrecv.StartMetricsOp(r.ctx)
	numPoints := metrics.DataPointCount()
	err = r.consumer.ConsumeMetrics(ctx, metrics)
	if err != nil {
		r.logger.Error("ConsumeMetricsData failed", zap.Error(err))
	}
	r.obsrecv.EndMetricsOp(ctx, typeStr, numPoints, err)

	return nil
}

func (r *runnable) detailedPVCLabelsSetter() func(volCacheID, volumeClaim, namespace string, labels map[string]string) error {
	return func(volCacheID, volumeClaim, namespace string, labels map[string]string) error {
		if r.k8sAPIClient == nil {
			return nil
		}

		if r.cachedVolumeLabels[volCacheID] == nil {
			ctx := context.Background()
			pvc, err := r.k8sAPIClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, volumeClaim, metav1.GetOptions{})
			if err != nil {
				return err
			}

			volName := pvc.Spec.VolumeName
			if volName == "" {
				return fmt.Errorf("PersistentVolumeClaim %s does not have a volume name", pvc.Name)
			}

			pv, err := r.k8sAPIClient.CoreV1().PersistentVolumes().Get(ctx, volName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			labelsToCache := make(map[string]string)
			kubelet.GetPersistentVolumeLabels(pv.Spec.PersistentVolumeSource, labelsToCache)

			// Cache collected labels.
			r.cachedVolumeLabels[volCacheID] = labelsToCache
		}

		for k, v := range r.cachedVolumeLabels[volCacheID] {
			labels[k] = v
		}
		return nil
	}
}
