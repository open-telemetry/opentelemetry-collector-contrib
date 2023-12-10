// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	cachedVolumeSource    map[string]v1.PersistentVolumeSource
	mbs                   *metadata.MetricsBuilders
	needsResources        bool
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
		cachedVolumeSource:    make(map[string]v1.PersistentVolumeSource),
		mbs: &metadata.MetricsBuilders{
			NodeMetricsBuilder:      metadata.NewMetricsBuilder(metricsConfig, set),
			PodMetricsBuilder:       metadata.NewMetricsBuilder(metricsConfig, set),
			ContainerMetricsBuilder: metadata.NewMetricsBuilder(metricsConfig, set),
			OtherMetricsBuilder:     metadata.NewMetricsBuilder(metricsConfig, set),
		},
		needsResources: metricsConfig.Metrics.K8sPodCPULimitUtilization.Enabled ||
			metricsConfig.Metrics.K8sPodCPURequestUtilization.Enabled ||
			metricsConfig.Metrics.K8sContainerCPULimitUtilization.Enabled ||
			metricsConfig.Metrics.K8sContainerCPURequestUtilization.Enabled ||
			metricsConfig.Metrics.K8sPodMemoryLimitUtilization.Enabled ||
			metricsConfig.Metrics.K8sPodMemoryRequestUtilization.Enabled ||
			metricsConfig.Metrics.K8sContainerMemoryLimitUtilization.Enabled ||
			metricsConfig.Metrics.K8sContainerMemoryRequestUtilization.Enabled,
	}
	return scraperhelper.NewScraper(metadata.Type, ks.scrape)
}

func (r *kubletScraper) scrape(context.Context) (pmetric.Metrics, error) {
	summary, err := r.statsProvider.StatsSummary()
	if err != nil {
		r.logger.Error("call to /stats/summary endpoint failed", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	var podsMetadata *v1.PodList
	// fetch metadata only when extra metadata labels are needed
	if len(r.extraMetadataLabels) > 0 || r.needsResources {
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

func (r *kubletScraper) detailedPVCLabelsSetter() func(rb *metadata.ResourceBuilder, volCacheID, volumeClaim, namespace string) error {
	return func(rb *metadata.ResourceBuilder, volCacheID, volumeClaim, namespace string) error {
		if r.k8sAPIClient == nil {
			return nil
		}

		if _, ok := r.cachedVolumeSource[volCacheID]; !ok {
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

			// Cache collected source.
			r.cachedVolumeSource[volCacheID] = pv.Spec.PersistentVolumeSource
		}
		kubelet.SetPersistentVolumeLabels(rb, r.cachedVolumeSource[volCacheID])
		return nil
	}
}
