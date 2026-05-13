// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/distribution/reference"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	clientmeta "k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"

	dcommon "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

// WatchClient is the main interface provided by this package to a kubernetes cluster.
type WatchClient struct {
	m                      sync.RWMutex
	deleteMut              sync.Mutex
	logger                 *zap.Logger
	kc                     kubernetes.Interface
	mc                     clientmeta.Interface
	informer               cache.SharedInformer
	namespaceInformer      cache.SharedInformer
	nodeInformer           cache.SharedInformer
	deploymentInformer     cache.SharedInformer
	statefulsetInformer    cache.SharedInformer
	daemonsetInformer      cache.SharedInformer
	jobInformer            cache.SharedInformer
	replicasetInformer     cache.SharedInformer
	cronJobRegex           *regexp.Regexp
	deleteQueue            []deleteRequest
	stopCh                 chan struct{}
	waitForMetadata        bool
	waitForMetadataTimeout time.Duration
	watchSyncPeriod        time.Duration

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

	// A map containing Node related data, used to associate them with resources.
	// Key is node name
	Nodes map[string]*Node

	// A map containing Deployment related data, used to associate them with resources.
	// Key is deployment uid
	Deployments map[string]*Deployment

	// A map containing StatefulSet related data, used to associate them with resources.
	// Key is statefulset uid
	StatefulSets map[string]*StatefulSet

	// A map containing DaemonSet related data, used to associate them with resources.
	// Key is daemonset uid
	DaemonSets map[string]*DaemonSet

	// A map containing job related data, used to associate them with resources.
	// Key is job uid
	Jobs map[string]*Job

	// A map containing ReplicaSets related data, used to associate them with resources.
	// Key is replicaset uid
	ReplicaSets map[string]*ReplicaSet

	telemetryBuilder *metadata.TelemetryBuilder
}

// Extract CronJob name from the Job name. Job name is created using
// format: [cronjob-name]-[time-hash-int]
// time-hash-int is the unix timestamp in minutes of the job creation time
// 8 digits will last until 2160, we are safe for a while
var cronJobRegex = regexp.MustCompile(`^(.*)-(\d{8})$`)

// cronJobSuffixTimeSkewMinutes is the maximum allowed difference
// in minutes between pod creation time and the timestamp suffix
// in the job name (created by cronjob)
// If the suffix falls outside of the range, it is assumed that it's not a cronjob
// (i.e. the suffix is human generated, like a date 20260407)
const cronJobSuffixTimeSkewMinutes int64 = 60 * 24 // 1 day

// podTemplateHashLabel contains the label that K8s Deployment
// controller adds to ReplicaSets to ensure that child ReplicaSets
// do not overlap. ReplicaSet name is constructed as [deployment-name]-[podTemplateHash]
// K8s doc ref: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#pod-template-hash-label
// K8s code ref: https://github.com/kubernetes/kubernetes/blob/b31119d205a839aab40b2d819a58d4fabacd9b47/pkg/controller/deployment/sync.go#L207
const podTemplateHashLabel = "pod-template-hash"

var errCannotRetrieveImage = errors.New("cannot retrieve image name")

type InformersFactoryList struct {
	newInformer           InformerProvider
	newNamespaceInformer  InformerProviderNamespace
	newReplicaSetInformer InformerProviderWorkload
}

// New initializes a new k8s Client.
func New(
	set component.TelemetrySettings,
	apiCfg k8sconfig.APIConfig,
	rules ExtractionRules,
	filters Filters,
	associations []Association,
	exclude Excludes,
	newClientSet APIClientsetProvider,
	informersFactory InformersFactoryList,
	waitForMetadata bool,
	waitForMetadataTimeout time.Duration,
	watchSyncPeriod time.Duration,
) (Client, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set)
	if err != nil {
		return nil, err
	}

	c := &WatchClient{
		logger:                 set.Logger,
		Rules:                  rules,
		Filters:                filters,
		Associations:           associations,
		Exclude:                exclude,
		cronJobRegex:           cronJobRegex,
		stopCh:                 make(chan struct{}),
		telemetryBuilder:       telemetryBuilder,
		waitForMetadata:        waitForMetadata,
		waitForMetadataTimeout: waitForMetadataTimeout,
		watchSyncPeriod:        watchSyncPeriod,
	}

	c.Pods = map[PodIdentifier]*Pod{}
	c.Namespaces = map[string]*Namespace{}
	c.Nodes = map[string]*Node{}
	c.ReplicaSets = map[string]*ReplicaSet{}
	c.Deployments = map[string]*Deployment{}
	c.StatefulSets = map[string]*StatefulSet{}
	c.DaemonSets = map[string]*DaemonSet{}
	c.Jobs = map[string]*Job{}

	if newClientSet == nil {
		newClientSet = k8sconfig.MakeClientBundle
	}
	bundle, err := newClientSet(apiCfg)
	if err != nil {
		return nil, err
	}
	c.kc = bundle.K8s
	c.mc = bundle.Meta

	labelSelector, fieldSelector, err := selectorsFromFilters(c.Filters)
	if err != nil {
		return nil, err
	}
	set.Logger.Info(
		"k8s filtering",
		zap.String("labelSelector", labelSelector.String()),
		zap.String("fieldSelector", fieldSelector.String()),
	)
	if informersFactory.newInformer == nil {
		informersFactory.newInformer = func(client kubernetes.Interface, ns string, ls labels.Selector, fs fields.Selector) cache.SharedInformer {
			return newSharedInformer(client, ns, ls, fs, watchSyncPeriod)
		}
	}

	if informersFactory.newNamespaceInformer == nil {
		switch {
		case c.extractNamespaceLabelsAnnotations():
			// if rules to extract metadata from namespace is configured use namespace shared informer containing
			// all namespaces including kube-system which contains cluster uid information (kube-system-uid)
			informersFactory.newNamespaceInformer = func(client clientmeta.Interface) cache.SharedInformer {
				return newNamespaceSharedInformer(client, watchSyncPeriod)
			}
		case rules.ClusterUID:
			// use kube-system shared informer to only watch kube-system namespace
			// reducing overhead of watching all the namespaces
			informersFactory.newNamespaceInformer = func(client clientmeta.Interface) cache.SharedInformer {
				return newKubeSystemSharedInformer(client, watchSyncPeriod)
			}
		default:
			informersFactory.newNamespaceInformer = NewNoOpInformer
		}
	}

	c.informer = informersFactory.newInformer(c.kc, c.Filters.Namespace, labelSelector, fieldSelector)
	err = c.informer.SetTransform(
		func(object any) (any, error) {
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

	c.namespaceInformer = informersFactory.newNamespaceInformer(c.mc)

	// Enable the ReplicaSet informer when any of the following applies:
	//   1) DeploymentName is enabled and DeploymentNameFromReplicaSet flag is false
	//   2) DeploymentUID is enabled
	//   3) Deployment labels or annotations are configured for extraction — those fields live on the Deployment
	//      resource, so we must associate Pods with Deployments through ReplicaSets first.
	needReplicaSetInformer := (c.Rules.DeploymentName && !c.Rules.DeploymentNameFromReplicaSet) ||
		c.Rules.DeploymentUID ||
		c.extractDeploymentLabelsAnnotations()

	if needReplicaSetInformer {
		if informersFactory.newReplicaSetInformer == nil {
			informersFactory.newReplicaSetInformer = func(client clientmeta.Interface, namespace string) cache.SharedInformer {
				return newReplicaSetSharedInformer(client, namespace, watchSyncPeriod)
			}
		}

		c.replicasetInformer = informersFactory.newReplicaSetInformer(c.mc, c.Filters.Namespace)
		if c.replicasetInformer == nil {
			return nil, errors.New("failed to create ReplicaSet informer")
		}
	}

	if c.extractNodeLabelsAnnotations() || c.extractNodeUID() {
		c.nodeInformer = newNodeSharedInformer(c.mc, c.Filters.Node, watchSyncPeriod)
	}

	if c.extractDeploymentLabelsAnnotations() {
		c.deploymentInformer = newDeploymentSharedInformer(c.mc, c.Filters.Namespace, watchSyncPeriod)
	}

	if c.extractStatefulSetLabelsAnnotations() {
		c.statefulsetInformer = newStatefulSetSharedInformer(c.mc, c.Filters.Namespace, watchSyncPeriod)
	}

	if c.extractDaemonSetLabelsAnnotations() {
		c.daemonsetInformer = newDaemonSetSharedInformer(c.mc, c.Filters.Namespace, watchSyncPeriod)
	}

	if c.extractJobLabelsAnnotations() || rules.CronJobUID {
		c.jobInformer = newJobSharedInformer(c.mc, c.Filters.Namespace, watchSyncPeriod)
	}
	return c, err
}

// Start registers pod event handlers and starts watching the kubernetes cluster for pod changes.
func (c *WatchClient) Start() error {
	// Start the delete loop for cleaning up old pods from cache
	go c.deleteLoop(time.Second*30, defaultPodDeleteGracePeriod)

	synced := make([]cache.InformerSynced, 0)
	// start the replicaSet informer first, as the replica sets need to be
	// present at the time the pods are handled, to correctly establish the connection between pods and deployments
	if c.replicasetInformer != nil {
		reg, err := c.replicasetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleReplicaSetAdd,
			UpdateFunc: c.handleReplicaSetUpdate,
			DeleteFunc: c.handleReplicaSetDelete,
		})
		if err != nil {
			return err
		}
		synced = append(synced, reg.HasSynced)
		go c.replicasetInformer.Run(c.stopCh)
	}

	reg, err := c.namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleNamespaceAdd,
		UpdateFunc: c.handleNamespaceUpdate,
		DeleteFunc: c.handleNamespaceDelete,
	})
	if err != nil {
		return err
	}
	synced = append(synced, reg.HasSynced)
	go c.namespaceInformer.Run(c.stopCh)

	if c.nodeInformer != nil {
		reg, err = c.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleNodeAdd,
			UpdateFunc: c.handleNodeUpdate,
			DeleteFunc: c.handleNodeDelete,
		})
		if err != nil {
			return err
		}
		synced = append(synced, reg.HasSynced)
		go c.nodeInformer.Run(c.stopCh)
	}

	if c.deploymentInformer != nil {
		reg, err = c.deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleDeploymentAdd,
			UpdateFunc: c.handleDeploymentUpdate,
			DeleteFunc: c.handleDeploymentDelete,
		})
		if err != nil {
			return err
		}
		synced = append(synced, reg.HasSynced)
		go c.deploymentInformer.Run(c.stopCh)
	}

	if c.statefulsetInformer != nil {
		reg, err = c.statefulsetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleStatefulSetAdd,
			UpdateFunc: c.handleStatefulSetUpdate,
			DeleteFunc: c.handleStatefulSetDelete,
		})
		if err != nil {
			return err
		}
		synced = append(synced, reg.HasSynced)
		go c.statefulsetInformer.Run(c.stopCh)
	}

	if c.daemonsetInformer != nil {
		reg, err = c.daemonsetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleDaemonSetAdd,
			UpdateFunc: c.handleDaemonSetUpdate,
			DeleteFunc: c.handleDaemonSetDelete,
		})
		if err != nil {
			return err
		}
		synced = append(synced, reg.HasSynced)
		go c.daemonsetInformer.Run(c.stopCh)
	}

	if c.jobInformer != nil {
		reg, err = c.jobInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleJobAdd,
			UpdateFunc: c.handleJobUpdate,
			DeleteFunc: c.handleJobDelete,
		})
		if err != nil {
			return err
		}
		synced = append(synced, reg.HasSynced)
		go c.jobInformer.Run(c.stopCh)
	}

	reg, err = c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handlePodAdd,
		UpdateFunc: c.handlePodUpdate,
		DeleteFunc: c.handlePodDelete,
	})
	if err != nil {
		return err
	}

	// start the podInformer with the prerequisite of the other informers to be finished first
	go c.runInformerWithDependencies(c.informer, synced)

	if c.waitForMetadata {
		timeoutCh := make(chan struct{})
		t := time.AfterFunc(c.waitForMetadataTimeout, func() {
			close(timeoutCh)
		})
		defer t.Stop()
		// Wait for the Pod informer to be completed.
		// The other informers will already be finished at this point, as the pod informer
		// waits for them be finished before it can run
		if !cache.WaitForCacheSync(timeoutCh, reg.HasSynced) {
			return errors.New("failed to wait for caches to sync")
		}
	}
	return nil
}

// Stop signals the k8s watcher/informer to stop watching for new events.
func (c *WatchClient) Stop() {
	close(c.stopCh)
}

func (c *WatchClient) handlePodAdd(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sPodAdded.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherPodAdded.Add(context.Background(), 1)
	}
	if pod, ok := obj.(*api_v1.Pod); ok {
		c.addOrUpdatePod(pod)
	} else {
		c.logger.Error("object received was not of type api_v1.Pod", zap.Any("received", obj))
	}
	podTableSize := len(c.Pods)
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sPodTableSize.Record(context.Background(), int64(podTableSize))
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherPodCacheSize.Record(context.Background(), int64(podTableSize))
	}
}

func (c *WatchClient) handlePodUpdate(_, newPod any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sPodUpdated.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherPodUpdated.Add(context.Background(), 1)
	}
	if pod, ok := newPod.(*api_v1.Pod); ok {
		// TODO: update or remove based on whether container is ready/unready?.
		c.addOrUpdatePod(pod)
	} else {
		c.logger.Error("object received was not of type api_v1.Pod", zap.Any("received", newPod))
	}
	podTableSize := len(c.Pods)
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sPodTableSize.Record(context.Background(), int64(podTableSize))
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherPodCacheSize.Record(context.Background(), int64(podTableSize))
	}
}

func (c *WatchClient) handlePodDelete(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sPodDeleted.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherPodDeleted.Add(context.Background(), 1)
	}
	if pod, ok := ignoreDeletedFinalStateUnknown(obj).(*api_v1.Pod); ok {
		c.forgetPod(pod)
	} else {
		c.logger.Error("object received was not of type api_v1.Pod", zap.Any("received", obj))
	}
	podTableSize := len(c.Pods)
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sPodTableSize.Record(context.Background(), int64(podTableSize))
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherPodCacheSize.Record(context.Background(), int64(podTableSize))
	}
}

func (c *WatchClient) handleNamespaceAdd(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sNamespaceAdded.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherNamespaceAdded.Add(context.Background(), 1)
	}
	if namespace, ok := obj.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateNamespace(namespace)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Namespace", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleNamespaceUpdate(_, newNamespace any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sNamespaceUpdated.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherNamespaceUpdated.Add(context.Background(), 1)
	}
	if namespace, ok := newNamespace.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateNamespace(namespace)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Namespace", zap.Any("received", newNamespace))
	}
}

func (c *WatchClient) handleNamespaceDelete(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sNamespaceDeleted.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherNamespaceDeleted.Add(context.Background(), 1)
	}
	if namespace, ok := ignoreDeletedFinalStateUnknown(obj).(*meta_v1.PartialObjectMetadata); ok {
		c.m.Lock()
		if ns, ok := c.Namespaces[namespace.Name]; ok {
			// When a namespace is deleted all the pods(and other k8s objects in that namespace) in that namespace are deleted before it.
			// So we wont have any spans that might need namespace annotations and labels.
			// Thats why we dont need an implementation for deleteQueue and gracePeriod for namespaces.
			delete(c.Namespaces, ns.Name)
		}
		c.m.Unlock()
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Namespace", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleNodeAdd(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sNodeAdded.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherNodeAdded.Add(context.Background(), 1)
	}
	if node, ok := obj.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateNode(node)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Node", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleNodeUpdate(_, newNode any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sNodeUpdated.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherNodeUpdated.Add(context.Background(), 1)
	}
	if node, ok := newNode.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateNode(node)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Node", zap.Any("received", newNode))
	}
}

func (c *WatchClient) handleNodeDelete(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sNodeDeleted.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherNodeDeleted.Add(context.Background(), 1)
	}
	if node, ok := ignoreDeletedFinalStateUnknown(obj).(*meta_v1.PartialObjectMetadata); ok {
		c.m.Lock()
		if n, ok := c.Nodes[node.Name]; ok {
			delete(c.Nodes, n.Name)
		}
		c.m.Unlock()
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Node", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleDeploymentAdd(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sDeploymentAdded.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherDeploymentAdded.Add(context.Background(), 1)
	}
	if deployment, ok := obj.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateDeployment(deployment)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Deployment", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleDeploymentUpdate(_, newDeployment any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sDeploymentUpdated.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherDeploymentUpdated.Add(context.Background(), 1)
	}
	if deployment, ok := newDeployment.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateDeployment(deployment)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Deployment", zap.Any("received", newDeployment))
	}
}

func (c *WatchClient) handleDeploymentDelete(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sDeploymentDeleted.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherDeploymentDeleted.Add(context.Background(), 1)
	}
	if deployment, ok := ignoreDeletedFinalStateUnknown(obj).(*meta_v1.PartialObjectMetadata); ok {
		c.m.Lock()
		if n, ok := c.Deployments[string(deployment.UID)]; ok {
			delete(c.Deployments, n.UID)
		}
		c.m.Unlock()
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Deployment", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleStatefulSetAdd(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sStatefulsetAdded.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherStatefulsetAdded.Add(context.Background(), 1)
	}
	if statefulset, ok := obj.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateStatefulSet(statefulset)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for StatefulSet", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleStatefulSetUpdate(_, newStatefulSet any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sStatefulsetUpdated.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherStatefulsetUpdated.Add(context.Background(), 1)
	}
	if statefulset, ok := newStatefulSet.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateStatefulSet(statefulset)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for StatefulSet", zap.Any("received", newStatefulSet))
	}
}

func (c *WatchClient) handleStatefulSetDelete(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sStatefulsetDeleted.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherStatefulsetDeleted.Add(context.Background(), 1)
	}
	if statefulset, ok := ignoreDeletedFinalStateUnknown(obj).(*meta_v1.PartialObjectMetadata); ok {
		c.m.Lock()
		if n, ok := c.StatefulSets[string(statefulset.UID)]; ok {
			delete(c.StatefulSets, n.UID)
		}
		c.m.Unlock()
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for StatefulSet", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleDaemonSetAdd(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sDaemonsetAdded.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherDaemonsetAdded.Add(context.Background(), 1)
	}
	if daemonset, ok := obj.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateDaemonSet(daemonset)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for DaemonSet", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleDaemonSetUpdate(_, newDaemonSet any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sDaemonsetUpdated.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherDaemonsetUpdated.Add(context.Background(), 1)
	}
	if daemonset, ok := newDaemonSet.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateDaemonSet(daemonset)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for DaemonSet", zap.Any("received", newDaemonSet))
	}
}

func (c *WatchClient) handleDaemonSetDelete(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sDaemonsetDeleted.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherDaemonsetDeleted.Add(context.Background(), 1)
	}
	if daemonset, ok := ignoreDeletedFinalStateUnknown(obj).(*meta_v1.PartialObjectMetadata); ok {
		c.m.Lock()
		if n, ok := c.DaemonSets[string(daemonset.UID)]; ok {
			delete(c.DaemonSets, n.UID)
		}
		c.m.Unlock()
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for DaemonSet", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleJobAdd(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sJobAdded.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherJobAdded.Add(context.Background(), 1)
	}
	if job, ok := obj.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateJob(job)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Job", zap.Any("received", obj))
	}
}

func (c *WatchClient) handleJobUpdate(_, newJob any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sJobUpdated.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherJobUpdated.Add(context.Background(), 1)
	}
	if job, ok := newJob.(*meta_v1.PartialObjectMetadata); ok {
		c.addOrUpdateJob(job)
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Job", zap.Any("received", newJob))
	}
}

func (c *WatchClient) handleJobDelete(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sJobDeleted.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherJobDeleted.Add(context.Background(), 1)
	}
	if job, ok := ignoreDeletedFinalStateUnknown(obj).(*meta_v1.PartialObjectMetadata); ok {
		c.m.Lock()
		if n, ok := c.Jobs[string(job.UID)]; ok {
			delete(c.Jobs, n.UID)
		}
		c.m.Unlock()
	} else {
		c.logger.Error("object received was not of type PartialObjectMetadata for Job", zap.Any("received", obj))
	}
}

func (c *WatchClient) deleteLoop(interval, gracePeriod time.Duration) {
	// This loop runs after N seconds and deletes pods from cache.
	// It iterates over the delete queue and deletes all that aren't
	// in the grace period anymore.
	for {
		select {
		case <-time.After(interval):
			c.deleteLoopProcessing(gracePeriod)
		case <-c.stopCh:
			return
		}
	}
}

func (c *WatchClient) deleteLoopProcessing(gracePeriod time.Duration) {
	var cutoff int
	now := time.Now()
	c.deleteMut.Lock()
	for i := range c.deleteQueue {
		d := c.deleteQueue[i]
		if d.ts.Add(gracePeriod).After(now) {
			break
		}
		cutoff = i + 1
	}
	toDelete := c.deleteQueue[:cutoff]
	c.deleteQueue = c.deleteQueue[cutoff:]
	c.deleteMut.Unlock()

	c.m.Lock()
	deleted := false
	for i := range toDelete {
		d := toDelete[i]
		if p, ok := c.Pods[d.id]; ok {
			// Sanity check: make sure we are deleting the same pod
			// and the underlying state (ip<>pod mapping) has not changed.
			if p.PodUID == d.podUID {
				delete(c.Pods, d.id)
				deleted = true
			}
		}
	}

	if deleted {
		c.compactPodMap()
	}

	podTableSize := len(c.Pods)
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sPodTableSize.Record(context.Background(), int64(podTableSize))
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherPodCacheSize.Record(context.Background(), int64(podTableSize))
	}
	c.m.Unlock()
}

func (c *WatchClient) compactPodMap() {
	newMap := make(map[PodIdentifier]*Pod, len(c.Pods))
	maps.Copy(newMap, c.Pods)
	c.Pods = newMap
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
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sIPLookupMiss.Add(context.Background(), 1)
	}
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

// GetNode takes a node name and returns the node object the node name is associated with.
func (c *WatchClient) GetNode(nodeName string) (*Node, bool) {
	c.m.RLock()
	node, ok := c.Nodes[nodeName]
	c.m.RUnlock()
	if ok {
		return node, ok
	}
	return nil, false
}

func (c *WatchClient) GetDeployment(deploymentUID string) (*Deployment, bool) {
	c.m.RLock()
	deployment, ok := c.Deployments[deploymentUID]
	c.m.RUnlock()
	if ok {
		return deployment, ok
	}
	return nil, false
}

func (c *WatchClient) GetReplicaSet(uid string) (*ReplicaSet, bool) {
	c.m.RLock()
	replicaset, ok := c.ReplicaSets[uid]
	c.m.RUnlock()
	if ok {
		return replicaset, ok
	}
	return nil, false
}

func (c *WatchClient) GetStatefulSet(statefulSetUID string) (*StatefulSet, bool) {
	c.m.RLock()
	statefulSet, ok := c.StatefulSets[statefulSetUID]
	c.m.RUnlock()
	if ok {
		return statefulSet, ok
	}
	return nil, false
}

func (c *WatchClient) GetDaemonSet(daemonSetUID string) (*DaemonSet, bool) {
	c.m.RLock()
	daemonSet, ok := c.DaemonSets[daemonSetUID]
	c.m.RUnlock()
	if ok {
		return daemonSet, ok
	}
	return nil, false
}

func (c *WatchClient) GetJob(jobUID string) (*Job, bool) {
	c.m.RLock()
	job, ok := c.Jobs[jobUID]
	c.m.RUnlock()
	if ok {
		return job, ok
	}
	return nil, false
}

func (c *WatchClient) extractPodAttributes(pod *api_v1.Pod) map[string]string {
	tags := map[string]string{}
	if c.Rules.PodName {
		tags[string(conventions.K8SPodNameKey)] = pod.Name
	}
	if c.Rules.ServiceName {
		tags[string(conventions.ServiceNameKey)] = pod.Name
	}

	if c.Rules.PodHostName {
		tags[string(conventions.K8SPodHostnameKey)] = pod.Spec.Hostname
	}

	if c.Rules.PodIP {
		tags[string(conventions.K8SPodIPKey)] = pod.Status.PodIP
	}

	if c.Rules.Namespace {
		tags[string(conventions.K8SNamespaceNameKey)] = pod.GetNamespace()
	}

	if c.Rules.ServiceNamespace {
		tags[string(conventions.ServiceNamespaceKey)] = pod.GetNamespace()
	}

	if c.Rules.StartTime {
		ts := pod.GetCreationTimestamp()
		if !ts.IsZero() {
			if rfc3339ts, err := ts.MarshalText(); err != nil {
				c.logger.Error("failed to unmarshal pod creation timestamp", zap.Error(err))
			} else {
				tags[string(conventions.K8SPodStartTimeKey)] = string(rfc3339ts)
			}
		}
	}

	if c.Rules.PodUID {
		uid := pod.GetUID()
		tags[string(conventions.K8SPodUIDKey)] = string(uid)
	}

	if c.Rules.ReplicaSetID || c.Rules.ReplicaSetName ||
		c.Rules.DaemonSetUID || c.Rules.DaemonSetName ||
		c.Rules.JobUID || c.Rules.JobName ||
		c.Rules.StatefulSetUID || c.Rules.StatefulSetName ||
		c.Rules.DeploymentName || c.Rules.DeploymentUID ||
		c.Rules.CronJobUID || c.Rules.CronJobName ||
		c.Rules.ServiceName {
		for _, ref := range pod.OwnerReferences {
			switch ref.Kind {
			case "ReplicaSet":
				if c.Rules.ReplicaSetID {
					tags[string(conventions.K8SReplicaSetUIDKey)] = string(ref.UID)
				}
				if c.Rules.ReplicaSetName {
					tags[string(conventions.K8SReplicaSetNameKey)] = ref.Name
				}
				if c.Rules.ServiceName {
					tags[string(conventions.ServiceNameKey)] = ref.Name
				}
				if c.Rules.DeploymentName || c.Rules.ServiceName {
					var deploymentName string
					// Attempt to use the ReplicaSet informer when it is running (e.g. DeploymentUID,
					// deployment_name_from_replicaset: false, or from: deployment label/annotation extraction).
					// Otherwise fall back to heuristics using the ReplicaSet name and pod-template-hash label.
					if replicaset, ok := c.GetReplicaSet(string(ref.UID)); ok {
						deploymentName = replicaset.Deployment.Name
						c.logger.Debug("Used ReplicaSet Informer to get deployment name", zap.String("replicaSetName", replicaset.Name), zap.String("deploymentName", replicaset.Deployment.Name))
					} else {
						// Will be blank if not a ReplicaSet managed by a Deployment
						deploymentName = extractDeploymentNameFromReplicaSet(ref.Name, pod.Labels[podTemplateHashLabel])
						c.logger.Debug("used heuristics for deployment name", zap.String("replicaSetName", ref.Name), zap.String("deploymentName", deploymentName))
					}

					if deploymentName != "" {
						if c.Rules.DeploymentName {
							tags[string(conventions.K8SDeploymentNameKey)] = deploymentName
						}
						if c.Rules.ServiceName {
							// deployment name wins over replicaset name
							tags[string(conventions.ServiceNameKey)] = deploymentName
						}
					}
				}
				if c.Rules.DeploymentUID {
					if replicaset, ok := c.GetReplicaSet(string(ref.UID)); ok {
						if replicaset.Deployment.UID != "" {
							tags[string(conventions.K8SDeploymentUIDKey)] = replicaset.Deployment.UID
						}
					}
				}

			case "DaemonSet":
				if c.Rules.DaemonSetUID {
					tags[string(conventions.K8SDaemonSetUIDKey)] = string(ref.UID)
				}
				if c.Rules.DaemonSetName {
					tags[string(conventions.K8SDaemonSetNameKey)] = ref.Name
				}
				if c.Rules.ServiceName {
					tags[string(conventions.ServiceNameKey)] = ref.Name
				}
			case "StatefulSet":
				if c.Rules.StatefulSetUID {
					tags[string(conventions.K8SStatefulSetUIDKey)] = string(ref.UID)
				}
				if c.Rules.StatefulSetName {
					tags[string(conventions.K8SStatefulSetNameKey)] = ref.Name
				}
				if c.Rules.ServiceName {
					tags[string(conventions.ServiceNameKey)] = ref.Name
				}
			case "Job":
				if c.Rules.JobUID {
					tags[string(conventions.K8SJobUIDKey)] = string(ref.UID)
				}
				if c.Rules.JobName {
					tags[string(conventions.K8SJobNameKey)] = ref.Name
				}
				if c.Rules.ServiceName {
					tags[string(conventions.ServiceNameKey)] = ref.Name
				}
				if c.Rules.CronJobName || c.Rules.ServiceName {
					var cronjobName string
					// Attempt to use the Job informer when it is running (e.g. k8s.cronjob.uid or from: job
					// label/annotation extraction). Otherwise fall back to heuristics on the Job name suffix.
					if job, ok := c.GetJob(string(ref.UID)); ok {
						cronjobName = job.CronJob.Name
						c.logger.Debug("used CronJob informer to get CronJob name", zap.String("cronjob", cronjobName))
					} else {
						cronjobName = c.extractCronJobNameFromJobOwner(ref, pod)
						c.logger.Debug("used heuristics for cronjob name", zap.String("cronjob", cronjobName))
					}

					if cronjobName != "" {
						if c.Rules.CronJobName {
							tags[string(conventions.K8SCronJobNameKey)] = cronjobName
						}
						if c.Rules.ServiceName {
							// cronjob name wins over job name
							tags[string(conventions.ServiceNameKey)] = cronjobName
						}
					}
				}
				if c.Rules.CronJobUID {
					if job, ok := c.GetJob(string(ref.UID)); ok {
						if job.CronJob.UID != "" {
							tags[string(conventions.K8SCronJobUIDKey)] = job.CronJob.UID
						}
					}
				}
			}
		}
	}

	if c.Rules.Node {
		tags[string(conventions.K8SNodeNameKey)] = pod.Spec.NodeName
	}

	if c.Rules.ClusterUID {
		if val, ok := c.GetNamespace("kube-system"); ok {
			tags[string(conventions.K8SClusterUIDKey)] = val.NamespaceUID
		} else {
			c.logger.Debug("unable to find kube-system namespace, cluster uid will not be available")
		}
	}

	enableStable := metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.IsEnabled()
	disableLegacy := metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.IsEnabled()

	for _, r := range c.Rules.Labels {
		if !disableLegacy {
			r.extractFromPodMetadata(pod.Labels, tags, K8SPodLabels)
		}
		if enableStable {
			r.extractFromPodMetadata(pod.Labels, tags, conventions.K8SPodLabel)
		}
	}

	if c.Rules.ServiceName {
		copyLabel(pod, tags, "app.kubernetes.io/name", conventions.ServiceNameKey)
		// app.kubernetes.io/instance has a higher precedence than app.kubernetes.io/name
		copyLabel(pod, tags, "app.kubernetes.io/instance", conventions.ServiceNameKey)
	}

	if c.Rules.ServiceVersion {
		copyLabel(pod, tags, "app.kubernetes.io/version", conventions.ServiceVersionKey)
	}

	// Annotations are processed after labels so that OTel annotations
	// (e.g. resource.opentelemetry.io/service.name) take precedence over
	// well-known Kubernetes labels (e.g. app.kubernetes.io/name).
	for _, r := range c.Rules.Annotations {
		if !disableLegacy {
			r.extractFromPodMetadata(pod.Annotations, tags, K8SPodAnnotations)
		}
		if enableStable {
			r.extractFromPodMetadata(pod.Annotations, tags, conventions.K8SPodAnnotation)
		}
	}

	return tags
}

func copyLabel(pod *api_v1.Pod, tags map[string]string, labelKey string, key attribute.Key) {
	if val, ok := pod.Labels[labelKey]; ok {
		tags[string(key)] = val
	}
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

	// Creation time is required for k8s.pod.start_time and for CronJob name heuristics
	// (Job name suffix vs pod creation) when k8s.pod.start_time is not extracted.
	if rules.StartTime || rules.CronJobName || rules.ServiceName {
		transformedPod.SetCreationTimestamp(pod.GetCreationTimestamp())
	}

	if rules.PodUID {
		transformedPod.SetUID(pod.GetUID())
	}

	if rules.Node || rules.NodeUID {
		transformedPod.Spec.NodeName = pod.Spec.NodeName
	}

	if rules.PodHostName {
		transformedPod.Spec.Hostname = pod.Spec.Hostname
	}

	if needContainerAttributes(rules) {
		removeUnnecessaryContainerStatus := func(c api_v1.ContainerStatus) api_v1.ContainerStatus {
			transformedContainerStatus := api_v1.ContainerStatus{
				Name:         c.Name,
				ContainerID:  c.ContainerID,
				RestartCount: c.RestartCount,
			}
			if rules.ContainerImageRepoDigests {
				transformedContainerStatus.ImageID = c.ImageID
			}
			return transformedContainerStatus
		}

		for i := range pod.Status.ContainerStatuses {
			containerStatus := pod.Status.ContainerStatuses[i]
			transformedPod.Status.ContainerStatuses = append(
				transformedPod.Status.ContainerStatuses, removeUnnecessaryContainerStatus(containerStatus),
			)
		}
		for i := range pod.Status.InitContainerStatuses {
			containerStatus := pod.Status.InitContainerStatuses[i]
			transformedPod.Status.InitContainerStatuses = append(
				transformedPod.Status.InitContainerStatuses, removeUnnecessaryContainerStatus(containerStatus),
			)
		}

		removeUnnecessaryContainerData := func(c api_v1.Container) api_v1.Container {
			transformedContainer := api_v1.Container{}
			transformedContainer.Name = c.Name // we always need the name, it's used for identification
			if rules.ContainerImageName || rules.ContainerImageTag || rules.ContainerImageTags || rules.ServiceVersion {
				transformedContainer.Image = c.Image
			}
			return transformedContainer
		}

		for i := range pod.Spec.Containers {
			container := pod.Spec.Containers[i]
			transformedPod.Spec.Containers = append(
				transformedPod.Spec.Containers, removeUnnecessaryContainerData(container),
			)
		}
		for i := range pod.Spec.InitContainers {
			container := pod.Spec.InitContainers[i]
			transformedPod.Spec.InitContainers = append(
				transformedPod.Spec.InitContainers, removeUnnecessaryContainerData(container),
			)
		}
	}

	if len(rules.Labels) > 0 || rules.ServiceName || rules.ServiceVersion || rules.DeploymentName {
		transformedPod.Labels = maps.Clone(pod.Labels)
	}

	if len(rules.Annotations) > 0 {
		transformedPod.Annotations = maps.Clone(pod.Annotations)
	}

	if rules.IncludesOwnerMetadata() {
		transformedPod.SetOwnerReferences(pod.GetOwnerReferences())
	}

	return &transformedPod
}

// parseServiceVersionFromImage parses the service version for differently-formatted image names
// according to https://github.com/open-telemetry/semantic-conventions/blob/main/docs/non-normative/k8s-attributes.md#how-serviceversion-should-be-calculated
func parseServiceVersionFromImage(image string) (string, error) {
	ref, err := reference.Parse(image)
	if err != nil {
		return "", err
	}

	namedRef, ok := ref.(reference.Named)
	if !ok {
		return "", errCannotRetrieveImage
	}
	var tag, digest string
	if taggedRef, ok := namedRef.(reference.Tagged); ok {
		tag = taggedRef.Tag()
	}
	if digestedRef, ok := namedRef.(reference.Digested); ok {
		digest = digestedRef.Digest().String()
	}
	if digest != "" {
		if tag != "" {
			return fmt.Sprintf("%s@%s", tag, digest), nil
		}
		return digest, nil
	}
	if tag != "" {
		return tag, nil
	}

	return "", errCannotRetrieveImage
}

func (c *WatchClient) extractPodContainersAttributes(pod *api_v1.Pod) PodContainers {
	containers := PodContainers{
		ByID:   map[string]*Container{},
		ByName: map[string]*Container{},
	}
	if !needContainerAttributes(c.Rules) {
		return containers
	}

	enableStable := metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.IsEnabled()
	disableLegacy := metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.IsEnabled()

	if c.Rules.ContainerImageName || c.Rules.ContainerImageTag || c.Rules.ContainerImageTags ||
		c.Rules.ServiceVersion || c.Rules.ServiceInstanceID {
		specs := append(pod.Spec.Containers, pod.Spec.InitContainers...) //nolint:gocritic // appendAssign: append result not assigned to the same slice
		for i := range specs {
			spec := &specs[i]
			container := &Container{}
			imageRef, err := dcommon.ParseImageName(spec.Image)
			if err == nil {
				if c.Rules.ContainerImageName {
					container.ImageName = imageRef.Repository
				}
				// Legacy: container.image.tag (singular, string)
				if c.Rules.ContainerImageTag && !disableLegacy {
					container.ImageTag = imageRef.Tag
				}
				// Stable: container.image.tags (plural, array)
				if c.Rules.ContainerImageTags && enableStable {
					container.ImageTags = []string{imageRef.Tag}
				}
				if c.Rules.ServiceVersion {
					serviceVersion, err := parseServiceVersionFromImage(spec.Image)
					if err == nil {
						container.ServiceVersion = serviceVersion
					}
				}
			}
			containers.ByName[spec.Name] = container
		}
	}
	apiStatuses := append(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...) //nolint:gocritic // appendAssign: append result not assigned to the same slice
	for i := range apiStatuses {
		apiStatus := &apiStatuses[i]
		containerName := apiStatus.Name
		container, ok := containers.ByName[containerName]
		if !ok {
			container = &Container{}
			containers.ByName[containerName] = container
		}
		if c.Rules.ContainerName {
			container.Name = containerName
		}
		if c.Rules.ServiceInstanceID {
			container.ServiceInstanceID = automaticServiceInstanceID(pod, containerName)
		}
		containerID := apiStatus.ContainerID
		// Remove container runtime prefix
		parts := strings.Split(containerID, "://")
		if len(parts) == 2 {
			containerID = parts[1]
		}
		containers.ByID[containerID] = container
		if c.Rules.ContainerID || c.Rules.ContainerImageRepoDigests {
			if container.Statuses == nil {
				container.Statuses = map[int]ContainerStatus{}
			}
			containerStatus := ContainerStatus{}
			if c.Rules.ContainerID {
				containerStatus.ContainerID = containerID
			}

			if c.Rules.ContainerImageRepoDigests {
				if canonicalRef, err := dcommon.CanonicalImageRef(apiStatus.ImageID); err == nil {
					containerStatus.ImageRepoDigest = canonicalRef
				}
			}

			container.Statuses[int(apiStatus.RestartCount)] = containerStatus
		}
	}
	return containers
}

func (c *WatchClient) extractNamespaceAttributes(namespace *meta_v1.PartialObjectMetadata) map[string]string {
	tags := map[string]string{}

	enableStable := metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.IsEnabled()
	disableLegacy := metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.IsEnabled()

	for _, r := range c.Rules.Labels {
		if !disableLegacy {
			r.extractFromNamespaceMetadata(namespace.Labels, tags, K8SNamespaceLabels)
		}
		if enableStable {
			r.extractFromNamespaceMetadata(namespace.Labels, tags, conventions.K8SNamespaceLabel)
		}
	}

	for _, r := range c.Rules.Annotations {
		if !disableLegacy {
			r.extractFromNamespaceMetadata(namespace.Annotations, tags, K8SNamespaceAnnotations)
		}
		if enableStable {
			r.extractFromNamespaceMetadata(namespace.Annotations, tags, conventions.K8SNamespaceAnnotation)
		}
	}

	return tags
}

func (c *WatchClient) extractNodeAttributes(node *meta_v1.PartialObjectMetadata) map[string]string {
	tags := map[string]string{}

	enableStable := metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.IsEnabled()
	disableLegacy := metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.IsEnabled()

	for _, r := range c.Rules.Labels {
		if !disableLegacy {
			r.extractFromNodeMetadata(node.Labels, tags, K8SNodeLabels)
		}
		if enableStable {
			r.extractFromNodeMetadata(node.Labels, tags, conventions.K8SNodeLabel)
		}
	}

	for _, r := range c.Rules.Annotations {
		if !disableLegacy {
			r.extractFromNodeMetadata(node.Annotations, tags, K8SNodeAnnotations)
		}
		if enableStable {
			r.extractFromNodeMetadata(node.Annotations, tags, conventions.K8SNodeAnnotation)
		}
	}
	return tags
}

func (c *WatchClient) extractDeploymentAttributes(d *meta_v1.PartialObjectMetadata) map[string]string {
	tags := map[string]string{}

	for _, r := range c.Rules.Labels {
		r.extractFromDeploymentMetadata(d.Labels, tags, conventions.K8SDeploymentLabel)
	}

	for _, r := range c.Rules.Annotations {
		r.extractFromDeploymentMetadata(d.Annotations, tags, conventions.K8SDeploymentAnnotation)
	}

	return tags
}

func (c *WatchClient) extractStatefulSetAttributes(d *meta_v1.PartialObjectMetadata) map[string]string {
	tags := map[string]string{}

	for _, r := range c.Rules.Labels {
		r.extractFromStatefulSetMetadata(d.Labels, tags, conventions.K8SStatefulSetLabel)
	}

	for _, r := range c.Rules.Annotations {
		r.extractFromStatefulSetMetadata(d.Annotations, tags, conventions.K8SStatefulSetAnnotation)
	}

	return tags
}

func (c *WatchClient) extractDaemonSetAttributes(d *meta_v1.PartialObjectMetadata) map[string]string {
	tags := map[string]string{}

	for _, r := range c.Rules.Labels {
		r.extractFromDaemonSetMetadata(d.Labels, tags, conventions.K8SDaemonSetLabel)
	}

	for _, r := range c.Rules.Annotations {
		r.extractFromDaemonSetMetadata(d.Annotations, tags, conventions.K8SDaemonSetAnnotation)
	}

	return tags
}

func (c *WatchClient) extractJobAttributes(d *meta_v1.PartialObjectMetadata) map[string]string {
	tags := map[string]string{}

	for _, r := range c.Rules.Labels {
		r.extractFromJobMetadata(d.Labels, tags, conventions.K8SJobLabel)
	}

	for _, r := range c.Rules.Annotations {
		r.extractFromJobMetadata(d.Annotations, tags, conventions.K8SJobAnnotation)
	}

	return tags
}

func (c *WatchClient) podFromAPI(pod *api_v1.Pod) *Pod {
	newPod := &Pod{
		Name:           pod.Name,
		Namespace:      pod.GetNamespace(),
		NodeName:       pod.Spec.NodeName,
		DeploymentUID:  "",
		StatefulSetUID: "",
		DaemonSetUID:   "",
		JobUID:         "",
		Address:        pod.Status.PodIP,
		HostNetwork:    pod.Spec.HostNetwork,
		PodUID:         string(pod.UID),
		StartTime:      pod.Status.StartTime,
	}

	if replicaset, ok := c.GetReplicaSet(getPodReplicaSetUID(pod)); ok {
		if replicaset.Deployment.UID != "" {
			newPod.DeploymentUID = replicaset.Deployment.UID
		}
	}

	if statefulset, ok := c.GetStatefulSet(getPodStatefulSetUID(pod)); ok {
		newPod.StatefulSetUID = statefulset.UID
	}

	if daemonset, ok := c.GetDaemonSet(getPodDaemonSetUID(pod)); ok {
		newPod.DaemonSetUID = daemonset.UID
	}

	if job, ok := c.GetJob(getPodJobUID(pod)); ok {
		newPod.JobUID = job.UID
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

func getPodReplicaSetUID(pod *api_v1.Pod) string {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "ReplicaSet" {
			return string(ref.UID)
		}
	}
	return ""
}

func getPodStatefulSetUID(pod *api_v1.Pod) string {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "StatefulSet" {
			return string(ref.UID)
		}
	}
	return ""
}

func getPodDaemonSetUID(pod *api_v1.Pod) string {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return string(ref.UID)
		}
	}
	return ""
}

func getPodJobUID(pod *api_v1.Pod) string {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "Job" {
			return string(ref.UID)
		}
	}
	return ""
}

// getIdentifiersFromAssoc returns list of PodIdentifiers for given pod
func (c *WatchClient) getIdentifiersFromAssoc(pod *Pod) []PodIdentifier {
	var ids []PodIdentifier
	for _, assoc := range c.Associations {
		retID4containerID := -1
		ret := PodIdentifier{}
		skip := false
		for i, source := range assoc.Sources {
			// If association configured to take IP address from connection
			switch source.From {
			case ConnectionSource:
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
			case ResourceSource:
				attr := ""
				switch source.Name {
				case string(conventions.K8SNamespaceNameKey):
					attr = pod.Namespace
				case string(conventions.K8SPodNameKey):
					attr = pod.Name
				case string(conventions.K8SPodUIDKey):
					attr = pod.PodUID
				case string(conventions.HostNameKey):
					attr = pod.Address
				// k8s.pod.ip is set by passthrough mode
				case string(conventions.K8SPodIPKey):
					attr = pod.Address
				case string(conventions.ContainerIDKey):
					// At this point just an empty attr is added and we remember the position.
					// Later this position in PodIdentifier will be filled with the actual
					// value for container.ID.
					retID4containerID = i
				default:
					if v, ok := pod.Attributes[source.Name]; ok {
						attr = v
					}
				}
				if attr == "" && retID4containerID == -1 {
					skip = true
					break
				}
				ret[i] = PodIdentifierAttributeFromSource(source, attr)
			}
		}

		if !skip {
			if retID4containerID != -1 {
				// As there can be multiple container.IDs per pod,
				// one PodIdentifier is added per container.ID.
				cIDs := maps.Keys(pod.Containers.ByID)
				for cID := range cIDs {
					retCpy := ret
					retCpy[retID4containerID] = PodIdentifierAttributeFromSource(AssociationSource{
						From: ResourceSource,
						Name: string(conventions.ContainerIDKey),
					}, cID)
					ids = append(ids, retCpy)
				}
			} else {
				ids = append(ids, ret)
			}
		}
	}

	// Ensure backward compatibility
	if pod.PodUID != "" {
		ids = append(ids, PodIdentifier{
			PodIdentifierAttributeFromResourceAttribute(string(conventions.K8SPodUIDKey), pod.PodUID),
		})
	}

	if pod.Address != "" && !pod.HostNetwork {
		ids = append(ids,
			PodIdentifier{
				PodIdentifierAttributeFromConnection(pod.Address),
			},
			// k8s.pod.ip is set by passthrough mode
			PodIdentifier{
				PodIdentifierAttributeFromResourceAttribute(string(conventions.K8SPodIPKey), pod.Address),
			})
	}

	return ids
}

func (c *WatchClient) addOrUpdatePod(pod *api_v1.Pod) {
	newPod := c.podFromAPI(pod)

	c.m.Lock()
	defer c.m.Unlock()

	identifiers := c.getIdentifiersFromAssoc(newPod)
	for i := range identifiers {
		id := identifiers[i]
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
	identifiers := c.getIdentifiersFromAssoc(podToRemove)
	for i := range identifiers {
		id := identifiers[i]
		p, ok := c.GetPod(id)

		if ok && p.PodUID == string(pod.UID) {
			c.appendDeleteQueue(id, p.PodUID)
		}
	}
}

func (c *WatchClient) appendDeleteQueue(podID PodIdentifier, podUID string) {
	c.deleteMut.Lock()
	c.deleteQueue = append(c.deleteQueue, deleteRequest{
		id:     podID,
		podUID: podUID,
		ts:     time.Now(),
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

var singleValueOperators = map[selection.Operator]int{
	selection.Equals:       1,
	selection.DoubleEquals: 1,
	selection.NotEquals:    1,
	selection.GreaterThan:  1,
	selection.LessThan:     1,
}

func selectorsFromFilters(filters Filters) (labels.Selector, fields.Selector, error) {
	labelSelector := labels.Everything()
	for _, f := range filters.Labels {
		if f.Op == selection.In || f.Op == selection.NotIn {
			return nil, nil, fmt.Errorf("label filters don't support operator: '%s'", f.Op)
		}

		var vals []string
		if _, ok := singleValueOperators[f.Op]; ok {
			vals = []string{f.Value}
		}

		r, err := labels.NewRequirement(f.Key, f.Op, vals)
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

func (c *WatchClient) addOrUpdateNamespace(namespace *meta_v1.PartialObjectMetadata) {
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

func (c *WatchClient) extractDeploymentLabelsAnnotations() bool {
	for _, r := range c.Rules.Labels {
		if r.From == MetadataFromDeployment {
			return true
		}
	}

	for _, r := range c.Rules.Annotations {
		if r.From == MetadataFromDeployment {
			return true
		}
	}

	return false
}

func (c *WatchClient) extractStatefulSetLabelsAnnotations() bool {
	for _, r := range c.Rules.Labels {
		if r.From == MetadataFromStatefulSet {
			return true
		}
	}

	for _, r := range c.Rules.Annotations {
		if r.From == MetadataFromStatefulSet {
			return true
		}
	}

	return false
}

func (c *WatchClient) extractDaemonSetLabelsAnnotations() bool {
	for _, r := range c.Rules.Labels {
		if r.From == MetadataFromDaemonSet {
			return true
		}
	}

	for _, r := range c.Rules.Annotations {
		if r.From == MetadataFromDaemonSet {
			return true
		}
	}

	return false
}

func (c *WatchClient) extractJobLabelsAnnotations() bool {
	for _, r := range c.Rules.Labels {
		if r.From == MetadataFromJob {
			return true
		}
	}

	for _, r := range c.Rules.Annotations {
		if r.From == MetadataFromJob {
			return true
		}
	}

	return false
}

func (c *WatchClient) extractNodeLabelsAnnotations() bool {
	for _, r := range c.Rules.Labels {
		if r.From == MetadataFromNode {
			return true
		}
	}

	for _, r := range c.Rules.Annotations {
		if r.From == MetadataFromNode {
			return true
		}
	}

	return false
}

func (c *WatchClient) extractNodeUID() bool {
	return c.Rules.NodeUID
}

func (c *WatchClient) addOrUpdateNode(node *meta_v1.PartialObjectMetadata) {
	newNode := &Node{
		Name:    node.Name,
		NodeUID: string(node.UID),
	}
	newNode.Attributes = c.extractNodeAttributes(node)

	c.m.Lock()
	if node.Name != "" {
		c.Nodes[node.Name] = newNode
	}
	c.m.Unlock()
}

func (c *WatchClient) addOrUpdateDeployment(deployment *meta_v1.PartialObjectMetadata) {
	newDeployment := &Deployment{
		Name: deployment.Name,
		UID:  string(deployment.UID),
	}
	newDeployment.Attributes = c.extractDeploymentAttributes(deployment)

	c.m.Lock()
	if deployment.UID != "" {
		c.Deployments[string(deployment.UID)] = newDeployment
	}
	c.m.Unlock()
}

func (c *WatchClient) addOrUpdateStatefulSet(statefulset *meta_v1.PartialObjectMetadata) {
	newStatefulSet := &StatefulSet{
		Name: statefulset.Name,
		UID:  string(statefulset.UID),
	}
	newStatefulSet.Attributes = c.extractStatefulSetAttributes(statefulset)

	c.m.Lock()
	if statefulset.UID != "" {
		c.StatefulSets[string(statefulset.UID)] = newStatefulSet
	}
	c.m.Unlock()
}

func (c *WatchClient) addOrUpdateDaemonSet(daemonset *meta_v1.PartialObjectMetadata) {
	newDaemonSet := &DaemonSet{
		Name: daemonset.Name,
		UID:  string(daemonset.UID),
	}
	newDaemonSet.Attributes = c.extractDaemonSetAttributes(daemonset)

	c.m.Lock()
	if daemonset.UID != "" {
		c.DaemonSets[string(daemonset.UID)] = newDaemonSet
	}
	c.m.Unlock()
}

func (c *WatchClient) addOrUpdateJob(job *meta_v1.PartialObjectMetadata) {
	newJob := &Job{
		Name: job.Name,
		UID:  string(job.UID),
	}
	newJob.Attributes = c.extractJobAttributes(job)

	for _, ownerReference := range job.OwnerReferences {
		if ownerReference.Kind == "CronJob" && ownerReference.Controller != nil && *ownerReference.Controller {
			newJob.CronJob = CronJob{
				Name: ownerReference.Name,
				UID:  string(ownerReference.UID),
			}
			break
		}
	}

	c.m.Lock()
	if job.UID != "" {
		c.Jobs[string(job.UID)] = newJob
	}
	c.m.Unlock()
}

func needContainerAttributes(rules ExtractionRules) bool {
	return rules.ContainerImageName ||
		rules.ContainerName ||
		rules.ContainerImageTag ||
		rules.ContainerImageTags ||
		rules.ContainerImageRepoDigests ||
		rules.ContainerID ||
		rules.ServiceVersion ||
		rules.ServiceInstanceID
}

func (c *WatchClient) handleReplicaSetAdd(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sReplicasetAdded.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherReplicasetAdded.Add(context.Background(), 1)
	}
	rsMeta, ok := obj.(*meta_v1.PartialObjectMetadata)
	if !ok {
		c.logger.Error("object received was not PartialObjectMetadata for ReplicaSet add", zap.Any("received", obj))
		return
	}
	c.addOrUpdateReplicaSet(rsMeta)
}

func (c *WatchClient) handleReplicaSetUpdate(_, newRS any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sReplicasetUpdated.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherReplicasetUpdated.Add(context.Background(), 1)
	}
	rsMeta, ok := newRS.(*meta_v1.PartialObjectMetadata)
	if !ok {
		c.logger.Error("object received was not PartialObjectMetadata for ReplicaSet add", zap.Any("received", newRS))
		return
	}
	c.addOrUpdateReplicaSet(rsMeta)
}

func (c *WatchClient) handleReplicaSetDelete(obj any) {
	if !metadata.ProcessorK8sattributesTelemetryDisableOldFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.OtelsvcK8sReplicasetDeleted.Add(context.Background(), 1)
	}
	if metadata.ProcessorK8sattributesTelemetryEnableNewFormatMetricsFeatureGate.IsEnabled() {
		c.telemetryBuilder.K8sWatcherReplicasetDeleted.Add(context.Background(), 1)
	}

	// Unwrap DeletedFinalStateUnknown if present
	o := ignoreDeletedFinalStateUnknown(obj)
	rsMeta, ok := o.(*meta_v1.PartialObjectMetadata)
	if !ok {
		c.logger.Error("object received was not a ReplicaSet (apps_v1 or PartialObjectMetadata)", zap.Any("received", obj))
	}
	uid := string(rsMeta.GetUID())
	if uid == "" {
		c.logger.Warn("received ReplicaSet delete without UID")
		return
	}

	c.m.Lock()
	delete(c.ReplicaSets, uid)
	c.m.Unlock()
}

func (c *WatchClient) addOrUpdateReplicaSet(replicaSet *meta_v1.PartialObjectMetadata) {
	uid := string(replicaSet.GetUID())
	newReplicaSet := &ReplicaSet{
		Name:      replicaSet.GetName(),
		Namespace: replicaSet.GetNamespace(),
		UID:       uid,
	}

	for _, owner := range replicaSet.GetOwnerReferences() {
		if owner.Kind == "Deployment" && owner.Controller != nil && *owner.Controller {
			newReplicaSet.Deployment = Deployment{
				Name: owner.Name,
				UID:  string(owner.UID),
			}
			break
		}
	}

	c.m.Lock()
	if uid != "" {
		c.ReplicaSets[uid] = newReplicaSet
	}
	c.m.Unlock()
}

// runInformerWithDependencies starts the given informer. The second argument is a list of other informers that should complete
// before the informer is started. This is necessary e.g. for the pod informer which requires the replica set informer
// to be finished to correctly establish the connection to the replicaset/deployment it belongs to.
func (c *WatchClient) runInformerWithDependencies(informer cache.SharedInformer, dependencies []cache.InformerSynced) {
	if len(dependencies) > 0 {
		timeoutCh := make(chan struct{})
		// TODO hard coding the timeout for now, check if we should make this configurable
		t := time.AfterFunc(5*time.Second, func() {
			close(timeoutCh)
		})
		defer t.Stop()
		cache.WaitForCacheSync(timeoutCh, dependencies...)
	}
	informer.Run(c.stopCh)
}

// ignoreDeletedFinalStateUnknown returns the object wrapped in
// DeletedFinalStateUnknown. Useful in OnDelete resource event handlers that do
// not need the additional context.
func ignoreDeletedFinalStateUnknown(obj any) any {
	if obj, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		return obj.Obj
	}
	return obj
}

func automaticServiceInstanceID(pod *api_v1.Pod, containerName string) string {
	resNames := []string{pod.Namespace, pod.Name, containerName}
	return strings.Join(resNames, ".")
}

// extractDeploymentNameFromReplicaSet attempts to extract deployment name from replicaset name
func extractDeploymentNameFromReplicaSet(replicasetName, podTemplateHash string) string {
	// TemplateHash Empty means it's not a ReplicaSet managed by a Deployment
	if replicasetName == "" || podTemplateHash == "" || !strings.HasSuffix(replicasetName, "-"+podTemplateHash) {
		return ""
	}
	// Remove the pod template hash suffix and the hyphen
	return replicasetName[:len(replicasetName)-len(podTemplateHash)-1]
}

// extractCronJobNameFromJobOwner returns the CronJob name for a Job owner reference using
// a heuristic that checks if the job name suffix is a valid timestamp closely matching the pod creation time.
// If it matches, then the suffix is assumed to be the job creation time, and the prefix is then the CronJob name.
func (c *WatchClient) extractCronJobNameFromJobOwner(ref meta_v1.OwnerReference, pod *api_v1.Pod) string {
	// Regex checks specifically for 8 digits at the end (Kubernetes CronJob job suffix is a
	// time value in minutes).
	parts := c.cronJobRegex.FindStringSubmatch(ref.Name)
	if len(parts) != 3 {
		return ""
	}
	suffixMinutes, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		c.logger.Debug("failed to parse cronjob timestamp", zap.Error(err))
		// assume not a cronjob
		return ""
	}
	podMinutes := pod.GetCreationTimestamp().Unix() / 60
	diff := podMinutes - suffixMinutes
	if diff < 0 {
		diff = -diff
	}
	if diff > cronJobSuffixTimeSkewMinutes {
		// assume not a cronjob
		return ""
	}

	return parts[1]
}
