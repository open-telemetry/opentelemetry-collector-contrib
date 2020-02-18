// Copyright 2019 Omnition Authors
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

package kube

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	api_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/observability"
)

// WatchClient is the main interface provided by this package to a kubernetes cluster.
type WatchClient struct {
	m               sync.RWMutex
	deleteMut       sync.Mutex
	logger          *zap.Logger
	kc              *kubernetes.Clientset
	informer        cache.SharedInformer
	deploymentRegex *regexp.Regexp
	deleteQueue     []deleteRequest
	stopCh          chan struct{}

	Pods    map[string]*Pod
	Rules   ExtractionRules
	Filters Filters
}

// New initializes a new k8s Client.
func New(logger *zap.Logger, rules ExtractionRules, filters Filters, newClientSet APIClientsetProvider, newInformer InformerProvider) (Client, error) {

	// Extract deployment name from the pod name. Pod name is created using
	// format: [deployment-name]-[Random-String-For-ReplicaSet]-[Random-String-For-Pod]
	dRegex, err := regexp.Compile(`^(.*)-[0-9a-zA-Z]*-[0-9a-zA-Z]*$`)
	if err != nil {
		return nil, err
	}
	c := &WatchClient{logger: logger, Rules: rules, Filters: filters, deploymentRegex: dRegex}
	go c.deleteLoop(time.Second * 30)

	c.Pods = map[string]*Pod{}
	if newClientSet == nil {
		newClientSet = newAPIClientset
	}

	kc, err := newClientSet()
	if err != nil {
		return nil, err
	}
	c.kc = kc

	labelSelector, fieldSelector, err := selectorsFromFilters(c.Filters)
	if err != nil {
		return nil, err
	}
	logger.Info(
		"k8s filtering",
		zap.String("labelSelector", labelSelector.String()),
		zap.String("fieldSelector", fieldSelector.String()),
	)
	if newInformer == nil {
		newInformer = newSharedInformer
	}

	c.informer = newInformer(c.kc, c.Filters.Namespace, labelSelector, fieldSelector)
	return c, err
}

// Start registers pod event handlers and starts watching the kubernetes cluster for pod changes.
func (c *WatchClient) Start() {
	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handlePodAdd,
		UpdateFunc: c.handlePodUpdate,
		DeleteFunc: c.handlePodDelete,
	})
	c.informer.Run(c.stopCh)
}

// Stop signals the the k8s watcher/informer to stop watching for new events.
func (c *WatchClient) Stop() {
	close(c.stopCh)
}

func (c *WatchClient) handlePodAdd(obj interface{}) {
	observability.RecordPodAdded()
	if pod, ok := obj.(*api_v1.Pod); ok {
		c.addOrUpdatePod(pod)
	} else {
		c.logger.Error("object received was not of type api_v1.Pod", zap.Any("received", obj))
	}
}

func (c *WatchClient) handlePodUpdate(old, new interface{}) {
	observability.RecordPodUpdated()
	if pod, ok := new.(*api_v1.Pod); ok {
		// TODO: update or remove based on whether container is ready/unready?.
		c.addOrUpdatePod(pod)
	} else {
		c.logger.Error("object received was not of type api_v1.Pod", zap.Any("received", new))
	}
}

func (c *WatchClient) handlePodDelete(obj interface{}) {
	observability.RecordPodDeleted()
	if pod, ok := obj.(*api_v1.Pod); ok {
		c.forgetPod(pod)
	} else {
		c.logger.Error("object received was not of type api_v1.Pod", zap.Any("received", obj))
	}
}

func (c *WatchClient) deleteLoop(interval time.Duration) {
	// TODO: if the gorountine crashes can it leave a mutex in locked state?
	// perhaps need to handle panics for this case?

	// This loop runs after N seconds and deletes pods from cache.
	// It iterates over the delete queue and deletes all that aren't
	// in the grace period anymore.
	for {
		<-time.After(interval)
		var cutoff int
		now := time.Now()
		c.deleteMut.Lock()
		for i, d := range c.deleteQueue {
			if d.ts.Add(podDeleteGracePeriod).After(now) {
				break
			}
			cutoff = i + 1
		}
		toDelete := c.deleteQueue[:cutoff]
		c.deleteQueue = c.deleteQueue[cutoff:]
		c.deleteMut.Unlock()

		c.m.Lock()
		for _, d := range toDelete {
			if p, ok := c.Pods[d.ip]; ok {
				// Sanity check: make sure we are deleting the same pod
				// and the underlying state (ip<>pod mapping) has not changed.
				if p.Name == d.name {
					delete(c.Pods, d.ip)
				}
			}
		}
		c.m.Unlock()
	}
}

// GetPodByIP takes an IP address and returns the pod the IP address is associated with.
func (c *WatchClient) GetPodByIP(ip string) (*Pod, bool) {
	c.m.RLock()
	pod, ok := c.Pods[ip]
	c.m.RUnlock()
	if ok {
		if pod.Ignore {
			return nil, false
		}
		return pod, ok
	}
	observability.RecordIPLookupMiss()
	return nil, false
}

func (c *WatchClient) extractPodAttributes(pod *api_v1.Pod) map[string]string {
	tags := map[string]string{}
	if c.Rules.PodName {
		tags[c.Rules.Tags.PodName] = pod.Name
	}

	if c.Rules.Namespace {
		tags[c.Rules.Tags.Namespace] = pod.GetNamespace()
	}

	if c.Rules.StartTime {
		ts := pod.GetCreationTimestamp()
		if !ts.IsZero() {
			tags[c.Rules.Tags.StartTime] = ts.String()
		}
	}

	if c.Rules.Deployment {
		// format: [deployment-name]-[Random-String-For-ReplicaSet]-[Random-String-For-Pod]
		parts := c.deploymentRegex.FindStringSubmatch(pod.Name)
		if len(parts) == 2 {
			tags[c.Rules.Tags.Deployment] = parts[1]
		}
	}

	if c.Rules.NodeName {
		tags[c.Rules.Tags.NodeName] = pod.Spec.NodeName
	}

	if c.Rules.HostName {
		tags[c.Rules.Tags.HostName] = pod.Spec.Hostname
	}

	if c.Rules.ClusterName {
		clusterName := pod.GetClusterName()
		if clusterName != "" {
			tags[c.Rules.Tags.ClusterName] = clusterName
		}
	}

	if c.Rules.Owners {
		for _, or := range pod.ObjectMeta.OwnerReferences {
			tags[fmt.Sprintf(c.Rules.Tags.OwnerTemplate, or.Kind)] = or.Name
		}
	}

	for _, r := range c.Rules.Labels {
		if v, ok := pod.Labels[r.Key]; ok {
			tags[r.Name] = c.extractField(v, r)
		}
	}

	for _, r := range c.Rules.Annotations {
		if v, ok := pod.Annotations[r.Key]; ok {
			tags[r.Name] = c.extractField(v, r)
		}
	}
	return tags
}

func (c *WatchClient) extractField(v string, r FieldExtractionRule) string {
	// Check if a subset of the field should be extracted with a regular expression
	// instead of the whole field.
	if r.Regex == nil {
		return v
	}

	matches := r.Regex.FindStringSubmatch(v)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func (c *WatchClient) addOrUpdatePod(pod *api_v1.Pod) {
	if pod.Status.PodIP == "" {
		return
	}

	c.m.Lock()
	defer c.m.Unlock()
	// compare initial scheduled timestamp for existing pod and new pod with same IP
	// and only replace old pod if scheduled time of new pod is newer? This should fix
	// the case where scheduler has assigned the same IP to a new pod but update event for
	// the old pod came in later
	if p, ok := c.Pods[pod.Status.PodIP]; ok {
		if p.StartTime != nil && pod.Status.StartTime.Before(p.StartTime) {
			return
		}
	}
	newPod := &Pod{
		Name:      pod.Name,
		Address:   pod.Status.PodIP,
		StartTime: pod.Status.StartTime,
	}

	if c.shouldIgnorePod(pod) {
		newPod.Ignore = true
	} else {
		newPod.Attributes = c.extractPodAttributes(pod)
	}
	c.Pods[pod.Status.PodIP] = newPod
}

func (c *WatchClient) forgetPod(pod *api_v1.Pod) {
	if pod.Status.PodIP == "" {
		return
	}
	c.m.RLock()
	p, ok := c.GetPodByIP(pod.Status.PodIP)
	c.m.RUnlock()

	if ok && p.Name == pod.Name {
		c.deleteMut.Lock()
		c.deleteQueue = append(c.deleteQueue, deleteRequest{
			ip:   pod.Status.PodIP,
			name: pod.Name,
			ts:   time.Now(),
		})
		c.deleteMut.Unlock()
	}
}

func (c *WatchClient) shouldIgnorePod(pod *api_v1.Pod) bool {
	// Host network mode is not supported right now with IP based
	// tagging as all pods in host network get same IP addresses.
	// Such pods are very rare and usually are used to monitor or control
	// host traffic (e.g, linkerd, flannel) instead of service business needs.
	// We plan to support host network pods in future.
	if pod.Spec.HostNetwork {
		return true
	}

	// Check if user requested the pod to be ignored through annotations
	if v, ok := pod.Annotations[ignoreAnnotation]; ok {
		if strings.ToLower(strings.TrimSpace(v)) == "true" {
			return true
		}
	}

	// Check well known names that should be ignored
	for _, rexp := range podNameIgnorePatterns {
		if rexp.MatchString(pod.Name) {
			return true
		}
	}

	return false
}

func selectorsFromFilters(filters Filters) (labels.Selector, fields.Selector, error) {
	labelSelector := labels.Everything()
	for _, f := range filters.Labels {
		r, err := labels.NewRequirement(f.Key, f.Op, []string{f.Value})
		if err != nil {
			return nil, nil, err
		}
		labelSelector = labelSelector.Add(*r)
	}

	var selectors []fields.Selector
	for _, f := range filters.Fields {
		switch f.Op {
		case selection.Equals:
			selectors = append(selectors, fields.OneTermEqualSelector(f.Key, f.Value))
		case selection.NotEquals:
			selectors = append(selectors, fields.OneTermNotEqualSelector(f.Key, f.Value))
		default:
			return nil, nil, fmt.Errorf("field filters don't support operator: '%s'", f.Op)
		}
	}

	if filters.Node != "" {
		selectors = append(selectors, fields.OneTermEqualSelector(podNodeField, filters.Node))
	}
	return labelSelector, fields.AndSelectors(selectors...), nil
}
