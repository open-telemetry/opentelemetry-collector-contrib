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

package kubeletstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

type scraperOptions struct {
	collectionInterval    time.Duration
	extraMetadataLabels   []kubelet.MetadataLabel
	metricGroupsToCollect map[kubelet.MetricGroup]bool
	k8sAPIClient          kubernetes.Interface
}

type kubletScraper struct {
	statsProvider         *kubelet.StatsProvider
	metadataProvider      *kubelet.MetadataProvider
	logger                *zap.Logger
	extraMetadataLabels   []kubelet.MetadataLabel
	metricGroupsToCollect map[kubelet.MetricGroup]bool
	k8sAPIClient          kubernetes.Interface
	cachedVolumeLabels    map[string][]metadata.ResourceMetricsOption
	mbs                   *metadata.MetricsBuilders
}

func newKubletScraper(
	restClient kubelet.RestClient,
	set receiver.CreateSettings,
	rOptions *scraperOptions,
	metricsConfig metadata.MetricsBuilderConfig,
) (scraperhelper.Scraper, error) {
	ks := &kubletScraper{
		statsProvider:         kubelet.NewStatsProvider(restClient),
		metadataProvider:      kubelet.NewMetadataProvider(restClient),
		logger:                set.Logger,
		extraMetadataLabels:   rOptions.extraMetadataLabels,
		metricGroupsToCollect: rOptions.metricGroupsToCollect,
		k8sAPIClient:          rOptions.k8sAPIClient,
		cachedVolumeLabels:    make(map[string][]metadata.ResourceMetricsOption),
		mbs: &metadata.MetricsBuilders{
			NodeMetricsBuilder:      metadata.NewMetricsBuilder(metricsConfig, set),
			PodMetricsBuilder:       metadata.NewMetricsBuilder(metricsConfig, set),
			ContainerMetricsBuilder: metadata.NewMetricsBuilder(metricsConfig, set),
			OtherMetricsBuilder:     metadata.NewMetricsBuilder(metricsConfig, set),
		},
	}
	return scraperhelper.NewScraper(typeStr, ks.scrape)
}

func (r *kubletScraper) scrape(context.Context) (pmetric.Metrics, error) {
	summary, err := r.statsProvider.StatsSummary()
	if err != nil {
		r.logger.Error("call to /stats/summary endpoint failed", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	var podsMetadata *v1.PodList
	// fetch metadata only when extra metadata labels are needed
	if len(r.extraMetadataLabels) > 0 {
		podsMetadata, err = r.metadataProvider.Pods()
		if err != nil {
			r.logger.Error("call to /pods endpoint failed", zap.Error(err))
			return pmetric.Metrics{}, err
		}
	}

	metadata := kubelet.NewMetadata(r.extraMetadataLabels, podsMetadata, r.detailedPVCLabelsSetter())
	mds := kubelet.MetricsData(r.logger, summary, metadata, r.metricGroupsToCollect, r.mbs)
	md := pmetric.NewMetrics()
	for i := range mds {
		mds[i].ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	}
	return md, nil
}

func (r *kubletScraper) detailedPVCLabelsSetter() func(volCacheID, volumeClaim, namespace string) ([]metadata.ResourceMetricsOption, error) {
	return func(volCacheID, volumeClaim, namespace string) ([]metadata.ResourceMetricsOption, error) {
		if r.k8sAPIClient == nil {
			return nil, nil
		}

		if r.cachedVolumeLabels[volCacheID] == nil {
			ctx := context.Background()
			pvc, err := r.k8sAPIClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, volumeClaim, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			volName := pvc.Spec.VolumeName
			if volName == "" {
				return nil, fmt.Errorf("PersistentVolumeClaim %s does not have a volume name", pvc.Name)
			}

			pv, err := r.k8sAPIClient.CoreV1().PersistentVolumes().Get(ctx, volName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			ro := kubelet.GetPersistentVolumeLabels(pv.Spec.PersistentVolumeSource)

			// Cache collected labels.
			r.cachedVolumeLabels[volCacheID] = ro
		}
		return r.cachedVolumeLabels[volCacheID], nil
	}
}
