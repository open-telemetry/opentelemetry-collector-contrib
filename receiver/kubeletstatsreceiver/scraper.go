// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
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
	nodeInformer          cache.SharedInformer
	stopCh                chan struct{}
	m                     sync.RWMutex

	// A struct that keeps Node's resource capacities
	nodeLimits *kubelet.NodeCapacity
}

func newKubletScraper(
	restClient kubelet.RestClient,
	set receiver.Settings,
	rOptions *scraperOptions,
	metricsConfig metadata.MetricsBuilderConfig,
	nodeName string,
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
		stopCh:     make(chan struct{}),
		nodeLimits: &kubelet.NodeCapacity{},
	}

	if metricsConfig.Metrics.K8sContainerCPUNodeUtilization.Enabled ||
		metricsConfig.Metrics.K8sPodCPUNodeUtilization.Enabled ||
		metricsConfig.Metrics.K8sContainerMemoryNodeUtilization.Enabled ||
		metricsConfig.Metrics.K8sPodMemoryNodeUtilization.Enabled {
		ks.nodeInformer = k8sconfig.NewNodeSharedInformer(rOptions.k8sAPIClient, nodeName, 5*time.Minute)
	}

	return scraperhelper.NewScraper(
		metadata.Type,
		ks.scrape,
		scraperhelper.WithStart(ks.start),
		scraperhelper.WithShutdown(ks.shutdown),
	)
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

	var node kubelet.NodeCapacity
	if r.nodeInformer != nil {
		node = r.node()
	}

	metaD := kubelet.NewMetadata(r.extraMetadataLabels, podsMetadata, node, r.detailedPVCLabelsSetter())

	mds := kubelet.MetricsData(r.logger, summary, metaD, r.metricGroupsToCollect, r.mbs)
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

func (r *kubletScraper) node() kubelet.NodeCapacity {
	r.m.RLock()
	defer r.m.RUnlock()
	return *r.nodeLimits
}

func (r *kubletScraper) start(_ context.Context, _ component.Host) error {
	if r.nodeInformer != nil {
		_, err := r.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    r.handleNodeAdd,
			UpdateFunc: r.handleNodeUpdate,
		})
		if err != nil {
			r.logger.Error("error adding event handler to node informer", zap.Error(err))
		}
		go r.nodeInformer.Run(r.stopCh)
	}
	return nil
}

func (r *kubletScraper) shutdown(_ context.Context) error {
	r.logger.Debug("executing close")
	if r.stopCh != nil {
		close(r.stopCh)
	}
	return nil
}

func (r *kubletScraper) handleNodeAdd(obj any) {
	if node, ok := obj.(*v1.Node); ok {
		r.addOrUpdateNode(node)
	} else {
		r.logger.Error("object received was not of type v1.Node", zap.Any("received", obj))
	}
}

func (r *kubletScraper) handleNodeUpdate(_, newNode any) {
	if node, ok := newNode.(*v1.Node); ok {
		r.addOrUpdateNode(node)
	} else {
		r.logger.Error("object received was not of type v1.Node", zap.Any("received", newNode))
	}
}

func (r *kubletScraper) addOrUpdateNode(node *v1.Node) {
	r.m.Lock()
	defer r.m.Unlock()

	if cpu, ok := node.Status.Capacity["cpu"]; ok {
		if q, err := resource.ParseQuantity(cpu.String()); err == nil {
			r.nodeLimits.CPUCapacity = float64(q.MilliValue()) / 1000
		}
	}
	if memory, ok := node.Status.Capacity["memory"]; ok {
		// ie: 32564740Ki
		if q, err := resource.ParseQuantity(memory.String()); err == nil {
			r.nodeLimits.MemoryCapacity = float64(q.Value())
		}
	}
}
