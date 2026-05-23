// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

var (
	_ extension.Extension = (*k8sObserver)(nil)
	_ observer.Observable = (*k8sObserver)(nil)
)

type k8sObserver struct {
	*endpointswatcher.EndpointsWatcher
	telemetry             component.TelemetrySettings
	podListerWatchers     []cache.ListerWatcher
	serviceListerWatchers []cache.ListerWatcher
	ingressListerWatchers []cache.ListerWatcher
	nodeListerWatcher     cache.ListerWatcher
	handler               *handler
	once                  *sync.Once
	stop                  chan struct{}
	config                *Config
}

// Start will populate the cache.SharedInformers for pods and nodes as configured and run them as goroutines.
func (k *k8sObserver) Start(_ context.Context, _ component.Host) error {
	if k.once == nil {
		return errors.New("cannot Start() partial k8sObserver (nil *sync.Once)")
	}
	if k.handler == nil {
		return errors.New("cannot Start() partial k8sObserver (nil *handler)")
	}

	k.once.Do(func() {
		if k.podListerWatchers != nil {
			for _, podListerWatcher := range k.podListerWatchers {
				k.telemetry.Logger.Debug("creating and starting pod informer")
				podInformer := cache.NewSharedInformer(podListerWatcher, &v1.Pod{}, 0)
				if _, err := podInformer.AddEventHandler(k.handler); err != nil {
					k.telemetry.Logger.Error("error adding event handler to pod informer", zap.Error(err))
				}
				go podInformer.Run(k.stop)
			}
		}
		if k.serviceListerWatchers != nil {
			for _, serviceListerWatcher := range k.serviceListerWatchers {
				k.telemetry.Logger.Debug("creating and starting service informer")
				serviceInformer := cache.NewSharedInformer(serviceListerWatcher, &v1.Service{}, 0)
				if _, err := serviceInformer.AddEventHandler(k.handler); err != nil {
					k.telemetry.Logger.Error("error adding event handler to service informer", zap.Error(err))
				}
				go serviceInformer.Run(k.stop)
			}
		}
		if k.nodeListerWatcher != nil {
			k.telemetry.Logger.Debug("creating and starting node informer")
			nodeInformer := cache.NewSharedInformer(k.nodeListerWatcher, &v1.Node{}, 0)
			go nodeInformer.Run(k.stop)
			if _, err := nodeInformer.AddEventHandler(k.handler); err != nil {
				k.telemetry.Logger.Error("error adding event handler to node informer", zap.Error(err))
			}
		}
		if k.ingressListerWatchers != nil {
			for _, ingressListerWatcher := range k.ingressListerWatchers {
				k.telemetry.Logger.Debug("creating and starting ingress informer")
				ingressInformer := cache.NewSharedInformer(ingressListerWatcher, &networkingv1.Ingress{}, 0)
				go ingressInformer.Run(k.stop)
				if _, err := ingressInformer.AddEventHandler(k.handler); err != nil {
					k.telemetry.Logger.Error("error adding event handler to ingress informer", zap.Error(err))
				}
			}
		}
	})
	return nil
}

// Shutdown tells any cache.SharedInformers to stop running.
func (k *k8sObserver) Shutdown(_ context.Context) error {
	close(k.stop)
	return nil
}

// newObserver creates a new k8s observer extension.
func newObserver(config *Config, set extension.Settings) (extension.Extension, error) {
	client, err := k8sconfig.MakeClient(config.APIConfig)
	if err != nil {
		return nil, err
	}
	restClient := client.CoreV1().RESTClient()

	var podListerWatchers []cache.ListerWatcher
	if config.ObservePods {
		var podSelector fields.Selector

		if config.Node == "" {
			podSelector = fields.Everything()
		} else {
			podSelector = fields.OneTermEqualSelector("spec.nodeName", config.Node)
		}
		set.Logger.Debug("observing pods")
		if len(config.Namespaces) == 0 {
			podListerWatchers = []cache.ListerWatcher{cache.NewListWatchFromClient(restClient, "pods", v1.NamespaceAll, podSelector)}
		} else {
			podListerWatchers = make([]cache.ListerWatcher, len(config.Namespaces))
			for i, namespace := range config.Namespaces {
				podListerWatchers[i] = cache.NewListWatchFromClient(restClient, "pods", namespace, podSelector)
			}
		}
	}

	var serviceListerWatchers []cache.ListerWatcher
	if config.ObserveServices {
		serviceSelector := fields.Everything()
		set.Logger.Debug("observing services")

		if len(config.Namespaces) == 0 {
			serviceListerWatchers = []cache.ListerWatcher{cache.NewListWatchFromClient(restClient, "services", v1.NamespaceAll, serviceSelector)}
		} else {
			serviceListerWatchers = make([]cache.ListerWatcher, len(config.Namespaces))
			for i, namespace := range config.Namespaces {
				serviceListerWatchers[i] = cache.NewListWatchFromClient(restClient, "services", namespace, serviceSelector)
			}
		}
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

	var ingressListerWatchers []cache.ListerWatcher
	if config.ObserveIngresses {
		ingressSelector := fields.Everything()
		set.Logger.Debug("observing ingresses")

		if len(config.Namespaces) == 0 {
			ingressListerWatchers = []cache.ListerWatcher{cache.NewListWatchFromClient(client.NetworkingV1().RESTClient(), "ingresses", v1.NamespaceAll, ingressSelector)}
		} else {
			ingressListerWatchers = make([]cache.ListerWatcher, len(config.Namespaces))
			for i, namespace := range config.Namespaces {
				ingressListerWatchers[i] = cache.NewListWatchFromClient(client.NetworkingV1().RESTClient(), "ingresses", namespace, ingressSelector)
			}
		}
	}
	h := &handler{idNamespace: set.ID.String(), endpoints: &sync.Map{}, logger: set.Logger}
	obs := &k8sObserver{
		EndpointsWatcher:      endpointswatcher.New(h, time.Second, set.Logger),
		telemetry:             set.TelemetrySettings,
		podListerWatchers:     podListerWatchers,
		serviceListerWatchers: serviceListerWatchers,
		nodeListerWatcher:     nodeListerWatcher,
		ingressListerWatchers: ingressListerWatchers,
		stop:                  make(chan struct{}),
		config:                config,
		handler:               h,
		once:                  &sync.Once{},
	}

	return obs, nil
}
