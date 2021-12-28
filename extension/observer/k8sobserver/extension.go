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

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

var _ component.Extension = (*k8sObserver)(nil)
var _ observer.Observable = (*k8sObserver)(nil)

type k8sObserver struct {
	telemetry         component.TelemetrySettings
	podInformer       cache.SharedInformer
	podListerWatcher  cache.ListerWatcher
	nodeInformer      cache.SharedInformer
	nodeListerWatcher cache.ListerWatcher
	stop              chan struct{}
	config            *Config
}

// Start will populate the cache.SharedInformers for pods and nodes as configured and run them as goroutines.
func (k *k8sObserver) Start(ctx context.Context, host component.Host) error {
	if k.podListerWatcher != nil && k.podInformer == nil {
		k.telemetry.Logger.Debug("creating and starting pod informer")
		k.podInformer = cache.NewSharedInformer(k.podListerWatcher, &v1.Pod{}, 0)
		go k.podInformer.Run(k.stop)
	}
	if k.nodeListerWatcher != nil && k.nodeInformer == nil {
		k.telemetry.Logger.Debug("creating and starting node informer")
		k.nodeInformer = cache.NewSharedInformer(k.nodeListerWatcher, &v1.Node{}, 0)
		go k.nodeInformer.Run(k.stop)
	}
	return nil
}

// Shutdown tells any cache.SharedInformers to stop running.
func (k *k8sObserver) Shutdown(ctx context.Context) error {
	close(k.stop)
	return nil
}

// ListAndWatch sets the respective cache.SharedInformer event handlers to inform the
// provided observer.Notify listener of pod and node entity updates
func (k *k8sObserver) ListAndWatch(listener observer.Notify) {
	if k.podInformer != nil {
		k.podInformer.AddEventHandler(&handler{listener: listener, idNamespace: k.config.ID().String(), logger: k.telemetry.Logger})
	}
	if k.nodeInformer != nil {
		k.nodeInformer.AddEventHandler(&handler{listener: listener, idNamespace: k.config.ID().String(), logger: k.telemetry.Logger})
	}
}

// newObserver creates a new k8s observer extension.
func newObserver(config *Config, telemetrySettings component.TelemetrySettings) (component.Extension, error) {
	client, err := k8sconfig.MakeClient(config.APIConfig)
	if err != nil {
		return nil, err
	}
	restClient := client.CoreV1().RESTClient()

	var podListerWatcher cache.ListerWatcher
	if config.ObservePods {
		var podSelector fields.Selector
		if config.Node == "" {
			podSelector = fields.Everything()
		} else {
			podSelector = fields.OneTermEqualSelector("spec.nodeName", config.Node)
		}
		telemetrySettings.Logger.Debug("observing pods")
		podListerWatcher = cache.NewListWatchFromClient(restClient, "pods", v1.NamespaceAll, podSelector)
	}

	var nodeListerWatcher cache.ListerWatcher
	if config.ObserveNodes {
		var nodeSelector fields.Selector
		if config.Node == "" {
			nodeSelector = fields.Everything()
		} else {
			nodeSelector = fields.OneTermEqualSelector("metadata.name", config.Node)
		}
		telemetrySettings.Logger.Debug("observing nodes")
		nodeListerWatcher = cache.NewListWatchFromClient(restClient, "nodes", v1.NamespaceAll, nodeSelector)
	}

	obs := &k8sObserver{
		telemetry:         telemetrySettings,
		podListerWatcher:  podListerWatcher,
		nodeListerWatcher: nodeListerWatcher,
		stop:              make(chan struct{}),
		config:            config,
	}

	return obs, nil
}
