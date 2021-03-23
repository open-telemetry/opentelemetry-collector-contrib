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

	// "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	CronJob = "CronJob"
)

type JobClient interface {
	JobToCronJob() map[string]string

	Init()
	Shutdown()
}

type jobClient struct {
	sync.RWMutex

	stopChan chan struct{}

	store *ObjStore

	inited bool

	cachedJobMap    map[string]time.Time
	jobToCronJobMap map[string]string
}

func (c *jobClient) JobToCronJob() map[string]string {
	if !c.inited {
		c.Init()
	}
	if c.store.Refreshed() {
		c.refresh()
	}
	c.RLock()
	defer c.RUnlock()
	return c.jobToCronJobMap
}

func (c *jobClient) refresh() {
	c.Lock()
	defer c.Unlock()

	objsList := c.store.List()

	tmpMap := make(map[string]string)
	for _, obj := range objsList {
		job := obj.(*jobInfo)
	ownerLoop:
		for _, owner := range job.owners {
			if owner.kind == CronJob && owner.name != "" {
				tmpMap[job.name] = owner.name
				break ownerLoop
			}
		}
	}

	if c.jobToCronJobMap == nil {
		c.jobToCronJobMap = make(map[string]string)
	}

	if c.cachedJobMap == nil {
		c.cachedJobMap = make(map[string]time.Time)
	}

	lastRefreshTime := time.Now()

	for k, v := range c.cachedJobMap {
		if lastRefreshTime.Sub(v) > cacheTTL {
			delete(c.jobToCronJobMap, k)
			delete(c.cachedJobMap, k)
		}
	}

	for k, v := range tmpMap {
		c.jobToCronJobMap[k] = v
		c.cachedJobMap[k] = lastRefreshTime
	}
}

func (c *jobClient) Init() {
	c.Lock()
	defer c.Unlock()
	if c.inited {
		return
	}

	ctx, _ := context.WithCancel(context.Background())
	if _, err := Get().ClientSet.BatchV1().Jobs(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		panic(fmt.Sprintf("Cannot list Job. err: %v", err))
	}

	c.stopChan = make(chan struct{})

	c.store = NewObjStore(transformFuncJob)

	lw := createJobListWatch(Get().ClientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &batchv1.Job{}, c.store, 0)
	go reflector.Run(c.stopChan)

	if err := wait.Poll(50*time.Millisecond, 2*time.Second, func() (done bool, err error) {
		return reflector.LastSyncResourceVersion() != "", nil
	}); err != nil {
		log.Printf("W! Job initial sync timeout: %v", err)
	}

	c.inited = true
}

func (c *jobClient) Shutdown() {
	c.Lock()
	defer c.Unlock()
	if !c.inited {
		return
	}

	close(c.stopChan)

	c.inited = false
}

func transformFuncJob(obj interface{}) (interface{}, error) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		return nil, errors.New(fmt.Sprintf("input obj %v is not Job type", obj))
	}
	info := new(jobInfo)
	info.name = job.Name
	info.owners = []*jobOwner{}
	for _, owner := range job.OwnerReferences {
		info.owners = append(info.owners, &jobOwner{kind: owner.Kind, name: owner.Name})
	}
	return info, nil
}

func createJobListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx, _ := context.WithCancel(context.Background())
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.BatchV1().Jobs(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.BatchV1().Jobs(ns).Watch(ctx, opts)
		},
	}
}
