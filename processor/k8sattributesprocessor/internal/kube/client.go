// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/featuregate"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	apps_v1 "k8s.io/api/apps/v1"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/observability"
)

// Upgrade to StageBeta in v0.83.0
var enableRFC3339Timestamp = featuregate.GlobalRegistry().MustRegister(
	"k8sattr.rfc3339",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, uses RFC3339 format for k8s.pod.start_time value"),
)

// WatchClient is the main interface provided by this package to a kubernetes cluster.
type WatchClient struct {
	m                  sync.RWMutex
	deleteMut          sync.Mutex
	logger             *zap.Logger
	kc                 kubernetes.Interface
	informer           cache.SharedInformer
	namespaceInformer  cache.SharedInformer
	replicasetInformer cache.SharedInformer
	replicasetRegex    *regexp.Regexp
	cronJobRegex       *regexp.Regexp
	deleteQueue        []deleteRequest
	stopCh             chan struct{}

	// A map containing Pod related data, used to associate them with resources.
	// Key can be either an IP address or Pod UID
	Pods         map[PodIdentifier]*Pod
	Rules        ExtractionRules
	Filters      Filters
	Associations []Association
	Exclude      Excludes

	// A map containing Namespace related data, used to associate them with resources.
	// Key is namespace name
	Namespaces map[string]*Namespace

	// A map containing ReplicaSets related data, used to associate them with resources.
	// Key is replicaset uid
	ReplicaSets map[string]*ReplicaSet
}

// Extract replicaset name from the pod name. Pod name is created using
// format: [deployment-name]-[Random-String-For-ReplicaSet]
var rRegex = regexp.MustCompile(`^(.*)-[0-9a-zA-Z]+$`)

// Extract CronJob name from the Job name. Job name is created using
// format: [cronjob-name]-[time-hash-int]
var cronJobRegex = regexp.MustCompile(`^(.*)-[0-9]+$`)

// New initializes a new k8s Client.
func New(logger *zap.Logger, apiCfg k8sconfig.APIConfig, rules ExtractionRules, filters Filters, associations []Association, exclude Excludes, newClientSet APIClientsetProvider, newInformer InformerProvider, newNamespaceInformer InformerProviderNamespace, newReplicaSetInformer InformerProviderReplicaSet) (Client, error) {
	c := &WatchClient{
		logger:          logger,
		Rules:           rules,
		Filters:         filters,
		Associations:    associations,
		Exclude:         exclude,
		replicasetRegex: rRegex,
		cronJobRegex:    cronJobRegex,
		stopCh:          make(chan struct{}),
	}
	go c.deleteLoop(time.Second*30, defaultPodDeleteGracePeriod)

	c.Pods = map[PodIdentifier]*Pod{}
	c.Namespaces = map[string]*Namespace{}
	c.ReplicaSets = map[string]*ReplicaSet{}
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
	logger.Info(
		"k8s filtering",
		zap.String("labelSelector", labelSelector.String()),
		zap.String("fieldSelector", fieldSelector.String()),
	)
	if newInformer == nil {
		newInformer = newSharedInformer
	}

	if newNamespaceInformer == nil {
		newNamespaceInformer = newNamespaceSharedInformer
	}

	c.informer = newInformer(c.kc, c.Filters.Namespace, labelSelector, fieldSelector)
	err = c.informer.SetTransform(
		func(object interface{}) (interface{}, error) {
			originalPod, success := object.(*api_v1.Pod)
			if !success { // means this is a cache.DeletedFinalStateUnknown, in which case we do nothing
				return object, nil
			}

			return removeUnnecessaryPodData(originalPod, c.Rules), nil
		},
	)
	if err != nil {
		return nil, err
	}

	if c.extractNamespaceLabelsAnnotations() {
		c.namespaceInformer = newNamespaceInformer(c.kc)
	} else {
		c.namespaceInformer = NewNoOpInformer(c.kc)
	}

	if rules.DeploymentName || rules.DeploymentUID {
		if newReplicaSetInformer == nil {
			newReplicaSetInformer = newReplicaSetSharedInformer
		}
		c.replicasetInformer = newReplicaSetInformer(c.kc, c.Filters.Namespace)
		err = c.replicasetInformer.SetTransform(
			func(object interface{}) (interface{}, error) {
				originalReplicaset, success := object.(*apps_v1.ReplicaSet)
				if !success { // means this is a cache.DeletedFinalStateUnknown, in which case we do nothing
					return object, nil
				}

				return removeUnnecessaryReplicaSetData(originalReplicaset), nil
			},
		)
		if err != nil {
			return nil, err
		}
	}

	return c, err
}

// Start registers pod event handlers and starts watching the kubernetes cluster for pod changes.
func (c *WatchClient) Start() {
	_, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handlePodAdd,
		UpdateFunc: c.handlePodUpdate,
		DeleteFunc: c.handlePodDelete,
	})
	if err != nil {
		c.logger.Error("error adding event handler to pod informer", zap.Error(err))
	}
	go c.informer.Run(c.stopCh)

	_, err = c.namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleNamespaceAdd,
		UpdateFunc: c.handleNamespaceUpdate,
		DeleteFunc: c.handleNamespaceDelete,
	})
	if err != nil {
		c.logger.Error("error adding event handler to namespace informer", zap.Error(err))
	}
	go c.namespaceInformer.Run(c.stopCh)

	if c.Rules.DeploymentName || c.Rules.DeploymentUID {
		_, err = c.replicasetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleReplicaSetAdd,
			UpdateFunc: c.handleReplicaSetUpdate,
			DeleteFunc: c.handleReplicaSetDelete,
		})
		if err != nil {
			c.logger.Error("error adding event handler to replicaset informer", zap.Error(err))
		}
		go c.replicasetInformer.Run(c.stopCh)
	}
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
	podTableSize := len(c.Pods)
	observability.RecordPodTableSize(int64(podTableSize))
}

func (c *WatchClient) handlePodUpdate(_, new interface{}) {
	observability.RecordPodUpdated()
	if pod, ok := new.(*api_v1.Pod); ok {
		// TODO: update or remove based on whether container is ready/unready?.
		c.addOrUpdatePod(pod)
	} else {
		c.logger.Error("object received was not of type api_v1.Pod", zap.Any("received", new))
	}
	podTableSize := len(c.Pods)
	observability.RecordPodTableSize(int64(podTableSize))
}

func (c *WatchClient) handlePodDelete(obj interface{}) {
	observability.RecordPodDeleted()
	if pod, ok := obj.(*api_v1.Pod); ok {
		c.forgetPod(pod)
	} else {
		c.logger.Error("object received was not of type api_v1.Pod", zap.Any("received", obj))
	}
	podTableSize := len(c.Pods)
	observability.RecordPodTableSize(int64(podTableSize))
}

func (c *WatchClient) handleNamespaceAdd(obj interface{}) {
	observability.RecordNamespaceAdded()
	if namespace, ok := obj.(*api_v1.Namespace); ok {
		c.addOrUpdateNamespace(namespace)
	} else {
		c.logger.Error("object received was not of type api_v1.Namespace", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleNamespaceUpdate(_, new interface{}) {
	observability.RecordNamespaceUpdated()
	if namespace, ok := new.(*api_v1.Namespace); ok {
		c.addOrUpdateNamespace(namespace)
	} else {
		c.logger.Error("object received was not of type api_v1.Namespace", zap.Any("received", new))
	}
}

func (c *WatchClient) handleNamespaceDelete(obj interface{}) {
	observability.RecordNamespaceDeleted()
	if namespace, ok := obj.(*api_v1.Namespace); ok {
		c.m.Lock()
		if ns, ok := c.Namespaces[namespace.Name]; ok {
			// When a namespace is deleted all the pods(and other k8s objects in that namespace) in that namespace are deleted before it.
			// So we wont have any spans that might need namespace annotations and labels.
			// Thats why we dont need an implementation for deleteQueue and gracePeriod for namespaces.
			delete(c.Namespaces, ns.Name)
		}
		c.m.Unlock()
	} else {
		c.logger.Error("object received was not of type api_v1.Namespace", zap.Any("received", obj))
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
				if p, ok := c.Pods[d.id]; ok {
					// Sanity check: make sure we are deleting the same pod
					// and the underlying state (ip<>pod mapping) has not changed.
					if p.Name == d.podName {
						delete(c.Pods, d.id)
					}
				}
			}
			podTableSize := len(c.Pods)
			observability.RecordPodTableSize(int64(podTableSize))
			c.m.Unlock()

		case <-c.stopCh:
			return
		}
	}
}

// GetPod takes an IP address or Pod UID and returns the pod the identifier is associated with.
func (c *WatchClient) GetPod(identifier PodIdentifier) (*Pod, bool) {
	c.m.RLock()
	pod, ok := c.Pods[identifier]
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

// GetNamespace takes a namespace and returns the namespace object the namespace is associated with.
func (c *WatchClient) GetNamespace(namespace string) (*Namespace, bool) {
	c.m.RLock()
	ns, ok := c.Namespaces[namespace]
	c.m.RUnlock()
	if ok {
		return ns, ok
	}
	return nil, false
}

func (c *WatchClient) extractPodAttributes(pod *api_v1.Pod) map[string]string {
	tags := map[string]string{}
	if c.Rules.PodName {
		tags[conventions.AttributeK8SPodName] = pod.Name
	}

	if c.Rules.PodHostName {
		tags[tagHostName] = pod.Spec.Hostname
	}

	if c.Rules.Namespace {
		tags[conventions.AttributeK8SNamespaceName] = pod.GetNamespace()
	}

	if c.Rules.StartTime {
		ts := pod.GetCreationTimestamp()
		if !ts.IsZero() {
			if enableRFC3339Timestamp.IsEnabled() {
				if rfc3339ts, err := ts.MarshalText(); err != nil {
					c.logger.Error("failed to unmarshal pod creation timestamp", zap.Error(err))
				} else {
					tags[tagStartTime] = string(rfc3339ts)
				}
			} else {
				tags[tagStartTime] = ts.String()
			}
		}
	}

	if c.Rules.PodUID {
		uid := pod.GetUID()
		tags[conventions.AttributeK8SPodUID] = string(uid)
	}

	if c.Rules.ReplicaSetID || c.Rules.ReplicaSetName ||
		c.Rules.DaemonSetUID || c.Rules.DaemonSetName ||
		c.Rules.JobUID || c.Rules.JobName ||
		c.Rules.StatefulSetUID || c.Rules.StatefulSetName ||
		c.Rules.DeploymentName || c.Rules.DeploymentUID ||
		c.Rules.CronJobName {
		for _, ref := range pod.OwnerReferences {
			switch ref.Kind {
			case "ReplicaSet":
				if c.Rules.ReplicaSetID {
					tags[conventions.AttributeK8SReplicaSetUID] = string(ref.UID)
				}
				if c.Rules.ReplicaSetName {
					tags[conventions.AttributeK8SReplicaSetName] = ref.Name
				}
				if c.Rules.DeploymentName {
					if replicaset, ok := c.getReplicaSet(string(ref.UID)); ok {
						if replicaset.Deployment.Name != "" {
							tags[conventions.AttributeK8SDeploymentName] = replicaset.Deployment.Name
						}
					}
				}
				if c.Rules.DeploymentUID {
					if replicaset, ok := c.getReplicaSet(string(ref.UID)); ok {
						if replicaset.Deployment.Name != "" {
							tags[conventions.AttributeK8SDeploymentUID] = replicaset.Deployment.UID
						}
					}
				}
			case "DaemonSet":
				if c.Rules.DaemonSetUID {
					tags[conventions.AttributeK8SDaemonSetUID] = string(ref.UID)
				}
				if c.Rules.DaemonSetName {
					tags[conventions.AttributeK8SDaemonSetName] = ref.Name
				}
			case "StatefulSet":
				if c.Rules.StatefulSetUID {
					tags[conventions.AttributeK8SStatefulSetUID] = string(ref.UID)
				}
				if c.Rules.StatefulSetName {
					tags[conventions.AttributeK8SStatefulSetName] = ref.Name
				}
			case "Job":
				if c.Rules.CronJobName {
					parts := c.cronJobRegex.FindStringSubmatch(ref.Name)
					if len(parts) == 2 {
						tags[conventions.AttributeK8SCronJobName] = parts[1]
					}
				}
				if c.Rules.JobUID {
					tags[conventions.AttributeK8SJobUID] = string(ref.UID)
				}
				if c.Rules.JobName {
					tags[conventions.AttributeK8SJobName] = ref.Name
				}
			}
		}
	}

	if c.Rules.Node {
		tags[tagNodeName] = pod.Spec.NodeName
	}

	for _, r := range c.Rules.Labels {
		r.extractFromPodMetadata(pod.Labels, tags, "k8s.pod.labels.%s")
	}

	for _, r := range c.Rules.Annotations {
		r.extractFromPodMetadata(pod.Annotations, tags, "k8s.pod.annotations.%s")
	}
	return tags
}

// This function removes all data from the Pod except what is required by extraction rules and pod association
func removeUnnecessaryPodData(pod *api_v1.Pod, rules ExtractionRules) *api_v1.Pod {

	// name, namespace, uid, start time and ip are needed for identifying Pods
	// there's room to optimize this further, it's kept this way for simplicity
	transformedPod := api_v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
			UID:       pod.GetUID(),
		},
		Status: api_v1.PodStatus{
			PodIP:     pod.Status.PodIP,
			StartTime: pod.Status.StartTime,
		},
		Spec: api_v1.PodSpec{
			HostNetwork: pod.Spec.HostNetwork,
		},
	}

	if rules.StartTime {
		transformedPod.SetCreationTimestamp(pod.GetCreationTimestamp())
	}

	if rules.PodUID {
		transformedPod.SetUID(pod.GetUID())
	}

	if rules.Node {
		transformedPod.Spec.NodeName = pod.Spec.NodeName
	}

	if rules.PodHostName {
		transformedPod.Spec.Hostname = pod.Spec.Hostname
	}

	if needContainerAttributes(rules) {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			transformedPod.Status.ContainerStatuses = append(
				transformedPod.Status.ContainerStatuses,
				api_v1.ContainerStatus{
					Name:         containerStatus.Name,
					ContainerID:  containerStatus.ContainerID,
					RestartCount: containerStatus.RestartCount,
				},
			)
		}
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			transformedPod.Status.InitContainerStatuses = append(
				transformedPod.Status.InitContainerStatuses,
				api_v1.ContainerStatus{
					Name:         containerStatus.Name,
					ContainerID:  containerStatus.ContainerID,
					RestartCount: containerStatus.RestartCount,
				},
			)
		}

		removeUnnecessaryContainerData := func(c api_v1.Container) api_v1.Container {
			transformedContainer := api_v1.Container{}
			transformedContainer.Name = c.Name // we always need the name, it's used for identification
			if rules.ContainerImageName || rules.ContainerImageTag {
				transformedContainer.Image = c.Image
			}
			return transformedContainer
		}

		for _, container := range pod.Spec.Containers {
			transformedPod.Spec.Containers = append(
				transformedPod.Spec.Containers, removeUnnecessaryContainerData(container),
			)
		}
		for _, container := range pod.Spec.InitContainers {
			transformedPod.Spec.InitContainers = append(
				transformedPod.Spec.InitContainers, removeUnnecessaryContainerData(container),
			)
		}
	}

	if len(rules.Labels) > 0 {
		transformedPod.Labels = pod.Labels
	}

	if len(rules.Annotations) > 0 {
		transformedPod.Annotations = pod.Annotations
	}

	if rules.IncludesOwnerMetadata() {
		transformedPod.SetOwnerReferences(pod.GetOwnerReferences())
	}

	return &transformedPod
}

func (c *WatchClient) extractPodContainersAttributes(pod *api_v1.Pod) PodContainers {
	containers := PodContainers{
		ByID:   map[string]*Container{},
		ByName: map[string]*Container{},
	}
	if !needContainerAttributes(c.Rules) {
		return containers
	}
	if c.Rules.ContainerImageName || c.Rules.ContainerImageTag {
		for _, spec := range append(pod.Spec.Containers, pod.Spec.InitContainers...) {
			container := &Container{}
			nameTagSep := strings.LastIndex(spec.Image, ":")
			if c.Rules.ContainerImageName {
				if nameTagSep > 0 {
					container.ImageName = spec.Image[:nameTagSep]
				} else {
					container.ImageName = spec.Image
				}
			}
			if c.Rules.ContainerImageTag && nameTagSep > 0 {
				container.ImageTag = spec.Image[nameTagSep+1:]
			}
			containers.ByName[spec.Name] = container
		}
	}
	for _, apiStatus := range append(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...) {
		container, ok := containers.ByName[apiStatus.Name]
		if !ok {
			container = &Container{}
			containers.ByName[apiStatus.Name] = container
		}
		if c.Rules.ContainerName {
			container.Name = apiStatus.Name
		}
		containerID := apiStatus.ContainerID
		// Remove container runtime prefix
		parts := strings.Split(containerID, "://")
		if len(parts) == 2 {
			containerID = parts[1]
		}
		containers.ByID[containerID] = container
		if c.Rules.ContainerID {
			if container.Statuses == nil {
				container.Statuses = map[int]ContainerStatus{}
			}
			container.Statuses[int(apiStatus.RestartCount)] = ContainerStatus{containerID}
		}
	}
	return containers
}

func (c *WatchClient) extractNamespaceAttributes(namespace *api_v1.Namespace) map[string]string {
	tags := map[string]string{}

	for _, r := range c.Rules.Labels {
		r.extractFromNamespaceMetadata(namespace.Labels, tags, "k8s.namespace.labels.%s")
	}

	for _, r := range c.Rules.Annotations {
		r.extractFromNamespaceMetadata(namespace.Annotations, tags, "k8s.namespace.annotations.%s")
	}

	return tags
}

func (c *WatchClient) podFromAPI(pod *api_v1.Pod) *Pod {
	newPod := &Pod{
		Name:        pod.Name,
		Namespace:   pod.GetNamespace(),
		Address:     pod.Status.PodIP,
		HostNetwork: pod.Spec.HostNetwork,
		PodUID:      string(pod.UID),
		StartTime:   pod.Status.StartTime,
	}

	if c.shouldIgnorePod(pod) {
		newPod.Ignore = true
	} else {
		newPod.Attributes = c.extractPodAttributes(pod)
		if needContainerAttributes(c.Rules) {
			newPod.Containers = c.extractPodContainersAttributes(pod)
		}
	}

	return newPod
}

// getIdentifiersFromAssoc returns list of PodIdentifiers for given pod
func (c *WatchClient) getIdentifiersFromAssoc(pod *Pod) []PodIdentifier {
	var ids []PodIdentifier
	for _, assoc := range c.Associations {
		ret := PodIdentifier{}
		skip := false
		for i, source := range assoc.Sources {
			// If association configured to take IP address from connection
			switch {
			case source.From == ConnectionSource:
				if pod.Address == "" {
					skip = true
					break
				}
				// Host network mode is not supported right now with IP based
				// tagging as all pods in host network get same IP addresses.
				// Such pods are very rare and usually are used to monitor or control
				// host traffic (e.g, linkerd, flannel) instead of service business needs.
				if pod.HostNetwork {
					skip = true
					break
				}
				ret[i] = PodIdentifierAttributeFromSource(source, pod.Address)
			case source.From == ResourceSource:
				attr := ""
				switch source.Name {
				case conventions.AttributeK8SNamespaceName:
					attr = pod.Namespace
				case conventions.AttributeK8SPodName:
					attr = pod.Name
				case conventions.AttributeK8SPodUID:
					attr = pod.PodUID
				case conventions.AttributeHostName:
					attr = pod.Address
				// k8s.pod.ip is set by passthrough mode
				case K8sIPLabelName:
					attr = pod.Address
				default:
					if v, ok := pod.Attributes[source.Name]; ok {
						attr = v
					}
				}

				if attr == "" {
					skip = true
					break
				}
				ret[i] = PodIdentifierAttributeFromSource(source, attr)
			}
		}

		if !skip {
			ids = append(ids, ret)
		}
	}

	// Ensure backward compatibility
	if pod.PodUID != "" {
		ids = append(ids, PodIdentifier{
			PodIdentifierAttributeFromResourceAttribute(conventions.AttributeK8SPodUID, pod.PodUID),
		})
	}

	if pod.Address != "" && !pod.HostNetwork {
		ids = append(ids, PodIdentifier{
			PodIdentifierAttributeFromConnection(pod.Address),
		})
		// k8s.pod.ip is set by passthrough mode
		ids = append(ids, PodIdentifier{
			PodIdentifierAttributeFromResourceAttribute(K8sIPLabelName, pod.Address),
		})
	}

	return ids
}

func (c *WatchClient) addOrUpdatePod(pod *api_v1.Pod) {
	newPod := c.podFromAPI(pod)

	c.m.Lock()
	defer c.m.Unlock()

	for _, id := range c.getIdentifiersFromAssoc(newPod) {
		// compare initial scheduled timestamp for existing pod and new pod with same identifier
		// and only replace old pod if scheduled time of new pod is newer or equal.
		// This should fix the case where scheduler has assigned the same attributes (like IP address)
		// to a new pod but update event for the old pod came in later.
		if p, ok := c.Pods[id]; ok {
			if pod.Status.StartTime.Before(p.StartTime) {
				continue
			}
		}
		c.Pods[id] = newPod
	}
}

func (c *WatchClient) forgetPod(pod *api_v1.Pod) {
	podToRemove := c.podFromAPI(pod)
	for _, id := range c.getIdentifiersFromAssoc(podToRemove) {
		p, ok := c.GetPod(id)

		if ok && p.Name == pod.Name {
			c.appendDeleteQueue(id, pod.Name)
		}
	}
}

func (c *WatchClient) appendDeleteQueue(podID PodIdentifier, podName string) {
	c.deleteMut.Lock()
	c.deleteQueue = append(c.deleteQueue, deleteRequest{
		id:      podID,
		podName: podName,
		ts:      time.Now(),
	})
	c.deleteMut.Unlock()
}

func (c *WatchClient) shouldIgnorePod(pod *api_v1.Pod) bool {
	// Check if user requested the pod to be ignored through annotations
	if v, ok := pod.Annotations[ignoreAnnotation]; ok {
		if strings.ToLower(strings.TrimSpace(v)) == "true" {
			return true
		}
	}

	// Check if user requested the pod to be ignored through configuration
	for _, excludedPod := range c.Exclude.Pods {
		if excludedPod.Name.MatchString(pod.Name) {
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
		case selection.DoesNotExist, selection.DoubleEquals, selection.In, selection.NotIn, selection.Exists, selection.GreaterThan, selection.LessThan:
			fallthrough
		default:
			return nil, nil, fmt.Errorf("field filters don't support operator: '%s'", f.Op)
		}
	}

	if filters.Node != "" {
		selectors = append(selectors, fields.OneTermEqualSelector(podNodeField, filters.Node))
	}
	return labelSelector, fields.AndSelectors(selectors...), nil
}

func (c *WatchClient) addOrUpdateNamespace(namespace *api_v1.Namespace) {
	newNamespace := &Namespace{
		Name:         namespace.Name,
		NamespaceUID: string(namespace.UID),
		StartTime:    namespace.GetCreationTimestamp(),
	}
	newNamespace.Attributes = c.extractNamespaceAttributes(namespace)

	c.m.Lock()
	if namespace.Name != "" {
		c.Namespaces[namespace.Name] = newNamespace
	}
	c.m.Unlock()
}

func (c *WatchClient) extractNamespaceLabelsAnnotations() bool {
	for _, r := range c.Rules.Labels {
		if r.From == MetadataFromNamespace {
			return true
		}
	}

	for _, r := range c.Rules.Annotations {
		if r.From == MetadataFromNamespace {
			return true
		}
	}

	return false
}

func needContainerAttributes(rules ExtractionRules) bool {
	return rules.ContainerImageName ||
		rules.ContainerName ||
		rules.ContainerImageTag ||
		rules.ContainerID
}

func (c *WatchClient) handleReplicaSetAdd(obj interface{}) {
	observability.RecordReplicaSetAdded()
	if replicaset, ok := obj.(*apps_v1.ReplicaSet); ok {
		c.addOrUpdateReplicaSet(replicaset)
	} else {
		c.logger.Error("object received was not of type apps_v1.ReplicaSet", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleReplicaSetUpdate(_, new interface{}) {
	observability.RecordReplicaSetUpdated()
	if replicaset, ok := new.(*apps_v1.ReplicaSet); ok {
		c.addOrUpdateReplicaSet(replicaset)
	} else {
		c.logger.Error("object received was not of type apps_v1.ReplicaSet", zap.Any("received", new))
	}
}

func (c *WatchClient) handleReplicaSetDelete(obj interface{}) {
	observability.RecordReplicaSetDeleted()
	if replicaset, ok := obj.(*apps_v1.ReplicaSet); ok {
		c.m.Lock()
		key := string(replicaset.UID)
		delete(c.ReplicaSets, key)
		c.m.Unlock()
	} else {
		c.logger.Error("object received was not of type apps_v1.ReplicaSet", zap.Any("received", obj))
	}
}

func (c *WatchClient) addOrUpdateReplicaSet(replicaset *apps_v1.ReplicaSet) {
	newReplicaSet := &ReplicaSet{
		Name:      replicaset.Name,
		Namespace: replicaset.Namespace,
		UID:       string(replicaset.UID),
	}

	for _, ownerReference := range replicaset.OwnerReferences {
		if ownerReference.Kind == "Deployment" && ownerReference.Controller != nil && *ownerReference.Controller {
			newReplicaSet.Deployment = Deployment{
				Name: ownerReference.Name,
				UID:  string(ownerReference.UID),
			}
			break
		}
	}

	c.m.Lock()
	if replicaset.UID != "" {
		c.ReplicaSets[string(replicaset.UID)] = newReplicaSet
	}
	c.m.Unlock()
}

// This function removes all data from the ReplicaSet except what is required by extraction rules
func removeUnnecessaryReplicaSetData(replicaset *apps_v1.ReplicaSet) *apps_v1.ReplicaSet {
	transformedReplicaset := apps_v1.ReplicaSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      replicaset.GetName(),
			Namespace: replicaset.GetNamespace(),
			UID:       replicaset.GetUID(),
		},
	}
	transformedReplicaset.SetOwnerReferences(replicaset.GetOwnerReferences())
	return &transformedReplicaset
}

func (c *WatchClient) getReplicaSet(uid string) (*ReplicaSet, bool) {
	c.m.RLock()
	replicaset, ok := c.ReplicaSets[uid]
	c.m.RUnlock()
	if ok {
		return replicaset, ok
	}
	return nil, false
}
