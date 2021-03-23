// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8scommon/k8sutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	TypePod = "Pod"
)

type Service struct {
	ServiceName string
	Namespace   string
}

func NewService(name, namespace string) Service {
	return Service{ServiceName: name, Namespace: namespace}
}

type EpClient interface {
	PodKeyToServiceNames() map[string][]string
	ServiceToPodNum() map[Service]int

	Init()
	Shutdown()
}

type epClient struct {
	sync.RWMutex

	stopChan chan struct{}
	store    *ObjStore

	inited bool

	podKeyToServiceNamesMap map[string][]string
	serviceToPodNumMap      map[Service]int //only running pods will show behind endpoints
}

func (c *epClient) PodKeyToServiceNames() map[string][]string {
	if !c.inited {
		c.Init()
	}
	if c.store.Refreshed() {
		c.refresh()
	}
	c.RLock()
	defer c.RUnlock()
	return c.podKeyToServiceNamesMap
}

func (c *epClient) ServiceToPodNum() map[Service]int {
	if !c.inited {
		c.Init()
	}
	if c.store.Refreshed() {
		c.refresh()
	}
	c.RLock()
	defer c.RUnlock()
	return c.serviceToPodNumMap
}

func (c *epClient) refresh() {
	c.Lock()
	defer c.Unlock()

	objsList := c.store.List()

	tmpMap := make(map[string]map[string]struct{}) //pod key to service names
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
			if serviceNamesMap, ok = tmpMap[podKey]; !ok {
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

func (c *epClient) Init() {
	c.Lock()
	defer c.Unlock()
	if c.inited {
		return
	}

	c.stopChan = make(chan struct{})

	c.store = NewObjStore(transformFuncEndpoint)

	lw := createEndpointListWatch(Get().ClientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &v1.Endpoints{}, c.store, 0)
	go reflector.Run(c.stopChan)

	if err := wait.Poll(50*time.Millisecond, 2*time.Second, func() (done bool, err error) {
		return reflector.LastSyncResourceVersion() != "", nil
	}); err != nil {
		log.Printf("W! Endpoint initial sync timeout: %v", err)
	}

	c.inited = true
}

func (c *epClient) Shutdown() {
	c.Lock()
	defer c.Unlock()
	if !c.inited {
		return
	}

	close(c.stopChan)

	c.inited = false
}

func transformFuncEndpoint(obj interface{}) (interface{}, error) {
	endpoint, ok := obj.(*v1.Endpoints)
	if !ok {
		return nil, errors.New(fmt.Sprintf("input obj %v is not Endpoint type", obj))
	}
	info := new(endpointInfo)
	info.name = endpoint.Name
	info.namespace = endpoint.Namespace
	info.podKeyList = []string{}
	if subsets := endpoint.Subsets; subsets != nil {
		for _, subset := range subsets {
			if addresses := subset.Addresses; addresses != nil {
				for _, address := range addresses {
					if targetRef := address.TargetRef; targetRef != nil && targetRef.Kind == TypePod {
						podKey := k8sutil.CreatePodKey(targetRef.Namespace, targetRef.Name)
						if podKey == "" {
							log.Printf("W! Invalid pod metadata, namespace: %s, podName: %s", targetRef.Namespace, targetRef.Name)
							continue
						}
						info.podKeyList = append(info.podKeyList, podKey)
					}
				}
			}
		}
	}
	return info, nil
}

func createEndpointListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx, _ := context.WithCancel(context.Background())
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.CoreV1().Endpoints(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.CoreV1().Endpoints(ns).Watch(ctx, opts)
		},
	}
}
