// Copyright 2020 OpenTelemetry Authors
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/observability"
)

// WatchClient is the main interface provided by this package to a kubernetes cluster.
type WatchClient struct {
	m               sync.RWMutex
	deleteMut       sync.Mutex
	logger          *zap.Logger
	kc              kubernetes.Interface
	informer        cache.SharedInformer
	deploymentRegex *regexp.Regexp
	deleteQueue     []deleteRequest
	stopCh          chan struct{}
	op              OwnerAPI

	Pods    map[string]*Pod
	Rules   ExtractionRules
	Filters Filters
}

// Extract deployment name from the pod name. Pod name is created using
// format: [deployment-name]-[Random-String-For-ReplicaSet]-[Random-String-For-Pod]
var dRegex = regexp.MustCompile(`^(.*)-[0-9a-zA-Z]*-[0-9a-zA-Z]*$`)

// New initializes a new k8s Client.
func New(logger *zap.Logger, apiCfg k8sconfig.APIConfig, rules ExtractionRules, filters Filters, newClientSet APIClientsetProvider, newInformer InformerProvider, newOwnerProviderFunc OwnerProvider) (Client, error) {
	c := &WatchClient{logger: logger, Rules: rules, Filters: filters, deploymentRegex: dRegex, stopCh: make(chan struct{})}
	go c.deleteLoop(time.Second*30, defaultPodDeleteGracePeriod)

	c.Pods = map[string]*Pod{}
	if newClientSet == nil {
		newClientSet = k8sconfig.MakeClient
	}

	kc, err := newClientSet(apiCfg)
	if err != nil {
		return nil, err
	}
	c.kc = kc

	labelSelector, fieldSelector, err := selectorsFromFilters(c.Filters)
	if err != nil {
		return nil, err
	}

	if c.Rules.OwnerLookupEnabled {
		if newOwnerProviderFunc == nil {
			newOwnerProviderFunc = newOwnerProvider
		}

		c.op, err = newOwnerProviderFunc(logger, c.kc, labelSelector, fieldSelector, c.Filters.Namespace)
		if err != nil {
			return nil, err
		}
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
	if c.op != nil {
		c.op.Start()
	}

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

	if c.op != nil {
		c.op.Stop()
	}
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

func (c *WatchClient) deleteLoop(interval time.Duration, gracePeriod time.Duration) {
	// This loop runs after N seconds and deletes pods from cache.
	// It iterates over the delete queue and deletes all that aren't
	// in the grace period anymore.
	for {
		select {
		case <-time.After(interval):
			var cutoff int
			now := time.Now()
			c.deleteMut.Lock()
			for i, d := range c.deleteQueue {
				if d.ts.Add(gracePeriod).After(now) {
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

		case <-c.stopCh:
			return
		}
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

	if c.Rules.PodUID {
		uid := pod.GetUID()
		tags[c.Rules.Tags.PodUID] = string(uid)
	}

	if c.Rules.DeploymentName {
		// format: [deployment-name]-[Random-String-For-ReplicaSet]-[Random-String-For-Pod]
		parts := c.deploymentRegex.FindStringSubmatch(pod.Name)
		if len(parts) == 2 {
			tags[c.Rules.Tags.DeploymentName] = parts[1]
		}
	}

	if c.Rules.NodeName {
		tags[c.Rules.Tags.NodeName] = pod.Spec.NodeName
	}

	if c.Rules.HostName {
		// Basing on v1.17 Kubernetes docs, when a hostname is specified, it takes precedence over
		// the associated metadata name, see:
		// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-hostname-and-subdomain-fields
		if pod.Spec.Hostname == "" {
			tags[c.Rules.Tags.HostName] = pod.Name
		} else {
			tags[c.Rules.Tags.HostName] = pod.Spec.Hostname
		}
	}

	if c.Rules.ClusterName {
		clusterName := pod.GetClusterName()
		if clusterName != "" {
			tags[c.Rules.Tags.ClusterName] = clusterName
		}
	}

	if c.Rules.OwnerLookupEnabled {
		owners := c.op.GetOwners(pod)

		for _, owner := range owners {
			switch owner.kind {
			case "DaemonSet":
				if c.Rules.DaemonSetName {
					tags[c.Rules.Tags.DaemonSetName] = owner.name
				}
			case "DeploymentName":
				// This should be already set earlier
			case "ReplicaSet":
				if c.Rules.ReplicaSetName {
					tags[c.Rules.Tags.ReplicaSetName] = owner.name
				}
			case "StatefulSet":
				if c.Rules.StatefulSetName {
					tags[c.Rules.Tags.StatefulSetName] = owner.name
				}
			default:
				// Do nothing
			}
		}

		if c.Rules.ServiceName {
			tags[c.Rules.Tags.ServiceName] = strings.Join(c.op.GetServices(pod), ", ")
		}

	}

	if len(pod.Status.ContainerStatuses) > 0 {
		cs := pod.Status.ContainerStatuses[0]
		if c.Rules.ContainerID {
			tags[c.Rules.Tags.ContainerID] = cs.ContainerID
		}
	}

	if len(pod.Spec.Containers) > 0 {
		container := pod.Spec.Containers[0]

		if c.Rules.ContainerName {
			tags[c.Rules.Tags.ContainerName] = container.Name
		}
		if c.Rules.ContainerImage {
			tags[c.Rules.Tags.ContainerImage] = container.Image
		}
	}

	if c.Rules.PodUID {
		tags[c.Rules.Tags.PodUID] = string(pod.UID)
	}

	for _, r := range c.Rules.Labels {
		c.extractLabelsIntoTags(r, pod.Labels, tags)
	}

	if len(c.Rules.NamespaceLabels) > 0 && c.Rules.OwnerLookupEnabled {
		namespace := c.op.GetNamespace(pod)
		if namespace != nil {
			for _, r := range c.Rules.NamespaceLabels {
				c.extractLabelsIntoTags(r, namespace.Labels, tags)
			}
		}
	}

	for _, r := range c.Rules.Annotations {
		c.extractLabelsIntoTags(r, pod.Annotations, tags)
	}
	return tags
}

func (c *WatchClient) extractLabelsIntoTags(r FieldExtractionRule, labels map[string]string, tags map[string]string) {
	if r.Key == "*" {
		// Special case, extract everything
		for label, value := range labels {
			tags[fmt.Sprintf(r.Name, label)] = c.extractField(value, r)
		}
	} else {
		if v, ok := labels[r.Key]; ok {
			tags[r.Name] = c.extractField(v, r)
		}
	}
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
