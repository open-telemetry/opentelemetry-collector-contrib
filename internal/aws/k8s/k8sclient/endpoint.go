// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sutil"
)

const (
	typePod = "Pod"
)

type Service struct {
	ServiceName string
	Namespace   string
}

func NewService(name, namespace string) Service {
	return Service{ServiceName: name, Namespace: namespace}
}

type EpClient interface {
	// Get the mapping between pod key and the corresponding service names
	PodKeyToServiceNames() map[string][]string
	// Get the mapping between the service and the number of belonging pods
	ServiceToPodNum() map[Service]int
}

type epClientOption func(*epClient)

func epSyncCheckerOption(checker initialSyncChecker) epClientOption {
	return func(e *epClient) {
		e.syncChecker = checker
	}
}

type epClient struct {
	stopChan chan struct{}
	store    *ObjStore

	stopped bool

	syncChecker initialSyncChecker

	mu                      sync.RWMutex
	podKeyToServiceNamesMap map[string][]string
	serviceToPodNumMap      map[Service]int // only running pods will show behind endpoints
}

func (c *epClient) PodKeyToServiceNames() map[string][]string {
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.podKeyToServiceNamesMap
}

func (c *epClient) ServiceToPodNum() map[Service]int {
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serviceToPodNumMap
}

func (c *epClient) refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()

	objsList := c.store.List()

	tmpMap := make(map[string]map[string]struct{}) // pod key to service names
	serviceToPodNumMapNew := make(map[Service]int)

	for _, obj := range objsList {
		ep := obj.(*endpointInfo)
		serviceName := ep.name
		namespace := ep.namespace

		// each obj should be a uniq service.
		// ignore the service which has 0 pods.
		if len(ep.podKeyList) > 0 {
			serviceToPodNumMapNew[NewService(serviceName, namespace)] = len(ep.podKeyList)
		}

		for _, podKey := range ep.podKeyList {
			var serviceNamesMap map[string]struct{}
			var ok bool
			if _, ok = tmpMap[podKey]; !ok {
				tmpMap[podKey] = make(map[string]struct{})
			}
			serviceNamesMap = tmpMap[podKey]
			serviceNamesMap[serviceName] = struct{}{}
		}
	}

	podKeyToServiceNamesMapNew := make(map[string][]string)

	for podKey, serviceNamesMap := range tmpMap {
		serviceNamesList := make([]string, 0, len(serviceNamesMap))
		for serviceName := range serviceNamesMap {
			serviceNamesList = append(serviceNamesList, serviceName)
		}
		podKeyToServiceNamesMapNew[podKey] = serviceNamesList
	}
	c.podKeyToServiceNamesMap = podKeyToServiceNamesMapNew
	c.serviceToPodNumMap = serviceToPodNumMapNew
}

func newEpClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...epClientOption) *epClient {
	c := &epClient{
		stopChan: make(chan struct{}),
	}

	for _, option := range options {
		option(c)
	}

	c.store = NewObjStore(transformFuncEndpoint, logger)
	lw := c.createEndpointListWatch(clientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &discoveryv1.EndpointSlice{}, c.store, 0)

	go reflector.Run(c.stopChan)

	if c.syncChecker != nil {
		// check the init sync for potential connection issue
		c.syncChecker.Check(reflector, "Endpoint initial sync timeout")
	}

	return c
}

func (c *epClient) shutdown() {
	close(c.stopChan)
	c.stopped = true
}

func transformFuncEndpoint(obj any) (any, error) {
	endpointSlice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not EndpointSlice type", obj)
	}
	info := new(endpointInfo)
	// EndpointSlice uses a label to reference the service
	if serviceName, ok := endpointSlice.Labels[discoveryv1.LabelServiceName]; ok {
		info.name = serviceName
	} else {
		// Fallback to the EndpointSlice name if label is not present
		info.name = endpointSlice.Name
	}
	info.namespace = endpointSlice.Namespace
	info.podKeyList = []string{}

	// EndpointSlice has Endpoints field (not Subsets like old Endpoints)
	for _, endpoint := range endpointSlice.Endpoints {
		// Check if endpoint is ready
		if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
			continue
		}

		if endpoint.TargetRef != nil && endpoint.TargetRef.Kind == typePod {
			podKey := k8sutil.CreatePodKey(endpoint.TargetRef.Namespace, endpoint.TargetRef.Name)
			if podKey == "" {
				continue
			}
			info.podKeyList = append(info.podKeyList, podKey)
		}
	}
	return info, nil
}

func (*epClient) createEndpointListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.DiscoveryV1().EndpointSlices(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.DiscoveryV1().EndpointSlices(ns).Watch(ctx, opts)
		},
	}
}
