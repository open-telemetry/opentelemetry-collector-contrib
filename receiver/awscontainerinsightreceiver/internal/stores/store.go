// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"context"
	"errors"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
)

var _ cadvisor.Decorator = &K8sDecorator{}

// CIMetric represents the raw metric interface for container insights
type CIMetric interface {
	HasField(key string) bool
	AddField(key string, val interface{})
	GetField(key string) interface{}
	HasTag(key string) bool
	AddTag(key, val string)
	GetTag(key string) string
	RemoveTag(key string)
}

type K8sStore interface {
	Decorate(ctx context.Context, metric CIMetric, kubernetesBlob map[string]interface{}) bool
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
}

func NewK8sDecorator(ctx context.Context, tagService bool, prefFullPodName bool, addFullPodNameMetricLabel bool, addContainerNameMetricLabel bool, includeEnhancedMetrics bool, logger *zap.Logger) (*K8sDecorator, error) {
	hostIP := os.Getenv("HOST_IP")
	if hostIP == "" {
		return nil, errors.New("environment variable HOST_IP is not set in k8s deployment config")
	}

	k := &K8sDecorator{
		ctx:                         ctx,
		addContainerNameMetricLabel: addContainerNameMetricLabel,
	}

	podstore, err := NewPodStore(hostIP, prefFullPodName, addFullPodNameMetricLabel, includeEnhancedMetrics, logger)
	if err != nil {
		return nil, err
	}
	k.stores = append(k.stores, podstore)

	if tagService {
		servicestore, err := NewServiceStore(logger)
		if err != nil {
			return nil, err
		}
		k.stores = append(k.stores, servicestore)
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

func (k *K8sDecorator) Decorate(metric *extractors.CAdvisorMetric) *extractors.CAdvisorMetric {
	kubernetesBlob := map[string]interface{}{}
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
