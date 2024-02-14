// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	cronJob = "CronJob"
)

type JobClient interface {
	// get the mapping between job and cronjob
	JobToCronJob() map[string]string
}

type noOpJobClient struct {
}

func (nc *noOpJobClient) JobToCronJob() map[string]string {
	return map[string]string{}
}

func (nc *noOpJobClient) shutdown() {
}

type jobClientOption func(*jobClient)

func jobSyncCheckerOption(checker initialSyncChecker) jobClientOption {
	return func(j *jobClient) {
		j.syncChecker = checker
	}
}

type jobClient struct {
	stopChan chan struct{}
	stopped  bool

	store *ObjStore

	syncChecker initialSyncChecker

	mu              sync.RWMutex
	cachedJobMap    map[string]time.Time
	jobToCronJobMap map[string]string
}

func (c *jobClient) JobToCronJob() map[string]string {
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.jobToCronJobMap
}

func (c *jobClient) refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()

	objsList := c.store.List()

	tmpMap := make(map[string]string)
	for _, obj := range objsList {
		job, ok := obj.(*jobInfo)
		if !ok {
			continue
		}
		for _, owner := range job.owners {
			if owner.kind == cronJob && owner.name != "" {
				tmpMap[job.name] = owner.name
				break
			}
		}
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

func newJobClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...jobClientOption) (*jobClient, error) {
	c := &jobClient{
		jobToCronJobMap: make(map[string]string),
		cachedJobMap:    make(map[string]time.Time),
		stopChan:        make(chan struct{}),
	}

	for _, option := range options {
		option(c)
	}

	ctx := context.Background()
	if _, err := clientSet.BatchV1().Jobs(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("cannot list Job. err: %w", err)
	}

	c.store = NewObjStore(transformFuncJob, logger)
	lw := createJobListWatch(clientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &batchv1.Job{}, c.store, 0)

	go reflector.Run(c.stopChan)

	if c.syncChecker != nil {
		// check the init sync for potential connection issue
		c.syncChecker.Check(reflector, "Job initial sync timeout")
	}

	return c, nil
}

func (c *jobClient) shutdown() {
	close(c.stopChan)
	c.stopped = true
}

func transformFuncJob(obj any) (any, error) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not Job type", obj)
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
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.BatchV1().Jobs(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.BatchV1().Jobs(ns).Watch(ctx, opts)
		},
	}
}
