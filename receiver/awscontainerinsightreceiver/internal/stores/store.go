// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores/kubeletutil"
)

// CIMetric represents the raw metric interface for container insights
type CIMetric interface {
	GetMetricType() string
	HasField(key string) bool
	AddField(key string, val any)
	GetField(key string) any
	GetFields() map[string]any
	HasTag(key string) bool
	AddTag(key, val string)
	GetTag(key string) string
	GetTags() map[string]string
	RemoveTag(key string)
}

type K8sStore interface {
	Decorate(ctx context.Context, metric CIMetric, kubernetesBlob map[string]any) bool
	RefreshTick(ctx context.Context)
}

type K8sDecorator struct {
	stores []K8sStore
	// We save ctx in the struct because it is used in Decorate(...) function when calling K8sStore.Decorate(...)
	// It would be easier to keep the ctx here than passing it as a parameter for Decorate(...) function.
	// The K8sStore (e.g. podstore) does network request in Decorate function, thus needs to take a context
	// object for canceling the request
	ctx                         context.Context
	addContainerNameMetricLabel bool
	// the pod store needs to be saved here because the map it is stateful and needs to be shut down.
	podStore *PodStore
}

func NewK8sDecorator(ctx context.Context, kubeletClient *kubeletutil.KubeletClient, tagService bool, prefFullPodName bool,
	addFullPodNameMetricLabel bool, addContainerNameMetricLabel bool, includeEnhancedMetrics bool, enableAcceleratedComputeMetrics bool,
	kubeConfigPath string, hostName string, isSystemd bool, logger *zap.Logger,
) (*K8sDecorator, error) {
	k := &K8sDecorator{
		ctx:                         ctx,
		addContainerNameMetricLabel: addContainerNameMetricLabel,
	}

	podstore, err := NewPodStore(kubeletClient, prefFullPodName, addFullPodNameMetricLabel, includeEnhancedMetrics, enableAcceleratedComputeMetrics, hostName, isSystemd, logger)
	if err != nil {
		return nil, err
	}
	k.podStore = podstore
	k.stores = append(k.stores, podstore)

	if tagService {
		servicestore, err := NewServiceStore(kubeConfigPath, logger)
		if err == nil {
			k.stores = append(k.stores, servicestore)
		}
	}

	go func() {
		refreshTicker := time.NewTicker(time.Second)
		for {
			select {
			case <-refreshTicker.C:
				for _, store := range k.stores {
					store.RefreshTick(k.ctx)
				}
			case <-k.ctx.Done():
				refreshTicker.Stop()
				return
			}
		}
	}()

	return k, nil
}

func (k *K8sDecorator) Decorate(metric CIMetric) CIMetric {
	kubernetesBlob := map[string]any{}
	for _, store := range k.stores {
		ok := store.Decorate(k.ctx, metric, kubernetesBlob)
		if !ok {
			return nil
		}
	}

	AddKubernetesInfo(metric, kubernetesBlob, k.addContainerNameMetricLabel)
	TagMetricSource(metric)
	return metric
}

func (k *K8sDecorator) Shutdown() error {
	return k.podStore.Shutdown()
}
