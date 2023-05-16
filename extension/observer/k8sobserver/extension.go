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

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

var _ extension.Extension = (*k8sObserver)(nil)
var _ observer.Observable = (*k8sObserver)(nil)

type k8sObserver struct {
	*observer.EndpointsWatcher
	telemetry         component.TelemetrySettings
	podListerWatcher  cache.ListerWatcher
	nodeListerWatcher cache.ListerWatcher
	handler           *handler
	once              *sync.Once
	stop              chan struct{}
	config            *Config
}

// Start will populate the cache.SharedInformers for pods and nodes as configured and run them as goroutines.
func (k *k8sObserver) Start(ctx context.Context, host component.Host) error {
	if k.once == nil {
		return fmt.Errorf("cannot Start() partial k8sObserver (nil *sync.Once)")
	}
	if k.handler == nil {
		return fmt.Errorf("cannot Start() partial k8sObserver (nil *handler)")
	}

	k.once.Do(func() {
		if k.podListerWatcher != nil {
			k.telemetry.Logger.Debug("creating and starting pod informer")
			podInformer := cache.NewSharedInformer(k.podListerWatcher, &v1.Pod{}, 0)
			if _, err := podInformer.AddEventHandler(k.handler); err != nil {
				k.telemetry.Logger.Error("error adding event handler to pod informer", zap.Error(err))
			}
			go podInformer.Run(k.stop)
		}
		if k.nodeListerWatcher != nil {
			k.telemetry.Logger.Debug("creating and starting node informer")
			nodeInformer := cache.NewSharedInformer(k.nodeListerWatcher, &v1.Node{}, 0)
			go nodeInformer.Run(k.stop)
			if _, err := nodeInformer.AddEventHandler(k.handler); err != nil {
				k.telemetry.Logger.Error("error adding event handler to node informer", zap.Error(err))
			}
		}
	})
	return nil
}

// Shutdown tells any cache.SharedInformers to stop running.
func (k *k8sObserver) Shutdown(ctx context.Context) error {
	close(k.stop)
	return nil
}

// newObserver creates a new k8s observer extension.
func newObserver(config *Config, set extension.CreateSettings) (extension.Extension, error) {
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
		set.Logger.Debug("observing pods")
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
		set.Logger.Debug("observing nodes")
		nodeListerWatcher = cache.NewListWatchFromClient(restClient, "nodes", v1.NamespaceAll, nodeSelector)
	}
	h := &handler{idNamespace: set.ID.String(), endpoints: &sync.Map{}, logger: set.TelemetrySettings.Logger}
	obs := &k8sObserver{
		EndpointsWatcher:  observer.NewEndpointsWatcher(h, time.Second, set.TelemetrySettings.Logger),
		telemetry:         set.TelemetrySettings,
		podListerWatcher:  podListerWatcher,
		nodeListerWatcher: nodeListerWatcher,
		stop:              make(chan struct{}),
		config:            config,
		handler:           h,
		once:              &sync.Once{},
	}

	return obs, nil
}
