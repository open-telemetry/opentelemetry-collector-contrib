// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
)

const (
	lockName = "otel-container-insight-clusterleader"
)

// eventBroadcaster is adpated from record.EventBroadcaster
type eventBroadcaster interface {
	// StartRecordingToSink starts sending events received from this EventBroadcaster to the given
	// sink. The return value can be ignored or used to stop recording, if desired.
	StartRecordingToSink(sink record.EventSink) watch.Interface
	// StartLogging starts sending events received from this EventBroadcaster to the given logging
	// function. The return value can be ignored or used to stop recording, if desired.
	StartLogging(logf func(format string, args ...any)) watch.Interface
	// NewRecorder returns an EventRecorder that can be used to send events to this EventBroadcaster
	// with the event source set to the given event source.
	NewRecorder(scheme *runtime.Scheme, source v1.EventSource) record.EventRecorder
}

type K8sClient interface {
	GetClientSet() kubernetes.Interface
	GetEpClient() k8sclient.EpClient
	GetNodeClient() k8sclient.NodeClient
	GetPodClient() k8sclient.PodClient
	ShutdownNodeClient()
	ShutdownPodClient()
}

// K8sAPIServer is a struct that produces metrics from kubernetes api server
type K8sAPIServer struct {
	nodeName            string // get the value from downward API
	logger              *zap.Logger
	clusterNameProvider clusterNameProvider
	cancel              context.CancelFunc

	mu      sync.Mutex
	leading bool

	k8sClient  K8sClient // *k8sclient.K8sClient
	epClient   k8sclient.EpClient
	nodeClient k8sclient.NodeClient
	podClient  k8sclient.PodClient

	// the following can be set to mocks in testing
	broadcaster eventBroadcaster
	// the close of isLeadingC indicates the leader election is done. This is used in testing
	isLeadingC chan bool
}

type clusterNameProvider interface {
	GetClusterName() string
}

type k8sAPIServerOption func(*K8sAPIServer)

// New creates a k8sApiServer which can generate cluster-level metrics
func New(clusterNameProvider clusterNameProvider, logger *zap.Logger, options ...k8sAPIServerOption) (*K8sAPIServer, error) {
	k := &K8sAPIServer{
		logger:              logger,
		clusterNameProvider: clusterNameProvider,
		k8sClient:           k8sclient.Get(logger),
		broadcaster:         record.NewBroadcaster(),
	}

	for _, opt := range options {
		opt(k)
	}

	if k.k8sClient == nil {
		return nil, errors.New("failed to start k8sapiserver because k8sclient is nil")
	}

	if err := k.init(); err != nil {
		return nil, fmt.Errorf("fail to initialize k8sapiserver, err: %w", err)
	}

	return k, nil
}

// GetMetrics returns an array of metrics
func (k *K8sAPIServer) GetMetrics() []pmetric.Metrics {
	var result []pmetric.Metrics

	// don't generate any metrics if the current collector is not the leader
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.leading {
		return result
	}

	// don't emit metrics if the cluster name is not detected
	clusterName := k.clusterNameProvider.GetClusterName()
	if clusterName == "" {
		k.logger.Warn("Failed to detect cluster name. Drop all metrics")
		return result
	}

	k.logger.Info("collect data from K8s API Server...")
	timestampNs := strconv.FormatInt(time.Now().UnixNano(), 10)

	fields := map[string]any{
		"cluster_failed_node_count": k.nodeClient.ClusterFailedNodeCount(),
		"cluster_node_count":        k.nodeClient.ClusterNodeCount(),
	}
	attributes := map[string]string{
		ci.ClusterNameKey: clusterName,
		ci.MetricType:     ci.TypeCluster,
		ci.Timestamp:      timestampNs,
		ci.Version:        "0",
	}
	if k.nodeName != "" {
		attributes["NodeName"] = k.nodeName
	}
	attributes[ci.SourcesKey] = "[\"apiserver\"]"
	md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
	result = append(result, md)

	for service, podNum := range k.epClient.ServiceToPodNum() {
		fields := map[string]any{
			"service_number_of_running_pods": podNum,
		}
		attributes := map[string]string{
			ci.ClusterNameKey: clusterName,
			ci.MetricType:     ci.TypeClusterService,
			ci.Timestamp:      timestampNs,
			ci.TypeService:    service.ServiceName,
			ci.K8sNamespace:   service.Namespace,
			ci.Version:        "0",
		}
		if k.nodeName != "" {
			attributes["NodeName"] = k.nodeName
		}
		attributes[ci.SourcesKey] = "[\"apiserver\"]"
		attributes[ci.Kubernetes] = fmt.Sprintf("{\"namespace_name\":\"%s\",\"service_name\":\"%s\"}",
			service.Namespace, service.ServiceName)
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		result = append(result, md)
	}

	for namespace, podNum := range k.podClient.NamespaceToRunningPodNum() {
		fields := map[string]any{
			"namespace_number_of_running_pods": podNum,
		}
		attributes := map[string]string{
			ci.ClusterNameKey: clusterName,
			ci.MetricType:     ci.TypeClusterNamespace,
			ci.Timestamp:      timestampNs,
			ci.K8sNamespace:   namespace,
			ci.Version:        "0",
		}
		if k.nodeName != "" {
			attributes["NodeName"] = k.nodeName
		}
		attributes[ci.SourcesKey] = "[\"apiserver\"]"
		attributes[ci.Kubernetes] = fmt.Sprintf("{\"namespace_name\":\"%s\"}", namespace)
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		result = append(result, md)
	}

	return result
}

func (k *K8sAPIServer) init() error {
	var ctx context.Context
	ctx, k.cancel = context.WithCancel(context.Background())

	k.nodeName = os.Getenv("HOST_NAME")
	if k.nodeName == "" {
		return errors.New("environment variable HOST_NAME is not set in k8s deployment config")
	}

	lockNamespace := os.Getenv("K8S_NAMESPACE")
	if lockNamespace == "" {
		return errors.New("environment variable K8S_NAMESPACE is not set in k8s deployment config")
	}

	clientSet := k.k8sClient.GetClientSet()
	configMapInterface := clientSet.CoreV1().ConfigMaps(lockNamespace)
	if configMap, err := configMapInterface.Get(ctx, lockName, metav1.GetOptions{}); configMap == nil || err != nil {
		k.logger.Info(fmt.Sprintf("Cannot get the leader config map: %v, try to create the config map...", err))
		configMap, err = configMapInterface.Create(ctx,
			&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: lockNamespace,
					Name:      lockName,
				},
			}, metav1.CreateOptions{})
		k.logger.Info(fmt.Sprintf("configMap: %v, err: %v", configMap, err))
	}

	lock, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		lockNamespace, lockName,
		clientSet.CoreV1(),
		clientSet.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      k.nodeName,
			EventRecorder: k.createRecorder(lockName, lockNamespace),
		})
	if err != nil {
		k.logger.Warn("Failed to create resource lock", zap.Error(err))
		return err
	}

	go k.startLeaderElection(ctx, lock)

	return nil
}

// Shutdown stops the k8sApiServer
func (k *K8sAPIServer) Shutdown() error {
	if k.cancel != nil {
		k.cancel()
	}
	return nil
}

func (k *K8sAPIServer) startLeaderElection(ctx context.Context, lock resourcelock.Interface) {

	for {
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock: lock,
			// IMPORTANT: you MUST ensure that any code you have that
			// is protected by the lease must terminate **before**
			// you call cancel. Otherwise, you could have a background
			// loop still running and another process could
			// get elected before your background loop finished, violating
			// the stated goal of the lease.
			LeaseDuration: 60 * time.Second,
			RenewDeadline: 15 * time.Second,
			RetryPeriod:   5 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					k.logger.Info(fmt.Sprintf("k8sapiserver OnStartedLeading: %s", k.nodeName))
					// we're notified when we start
					k.mu.Lock()
					k.leading = true
					// always retrieve clients in case previous ones shut down during leader switching
					k.nodeClient = k.k8sClient.GetNodeClient()
					k.podClient = k.k8sClient.GetPodClient()
					k.epClient = k.k8sClient.GetEpClient()
					k.mu.Unlock()

					if k.isLeadingC != nil {
						// this executes only in testing
						close(k.isLeadingC)
					}

					for {
						k.mu.Lock()
						leading := k.leading
						k.mu.Unlock()
						if !leading {
							k.logger.Info("no longer leading")
							return
						}
						select {
						case <-ctx.Done():
							k.logger.Info("ctx cancelled")
							return
						case <-time.After(time.Second):
						}
					}
				},
				OnStoppedLeading: func() {
					k.logger.Info(fmt.Sprintf("k8sapiserver OnStoppedLeading: %s", k.nodeName))
					// we can do cleanup here, or after the RunOrDie method returns
					k.mu.Lock()
					defer k.mu.Unlock()
					k.leading = false
					// node and pod are only used for cluster level metrics, endpoint is used for decorator too.
					k.k8sClient.ShutdownNodeClient()
					k.k8sClient.ShutdownPodClient()
				},
				OnNewLeader: func(identity string) {
					k.logger.Info(fmt.Sprintf("k8sapiserver Switch New Leader: %s", identity))
				},
			},
		})

		select {
		case <-ctx.Done(): // when leader election ends, the channel ctx.Done() will be closed
			k.logger.Info(fmt.Sprintf("k8sapiserver shutdown Leader Election: %s", k.nodeName))
			return
		default:
		}
	}
}

func (k *K8sAPIServer) createRecorder(name, namespace string) record.EventRecorder {
	k.broadcaster.StartLogging(klog.Infof)
	clientSet := k.k8sClient.GetClientSet()
	k.broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: corev1.New(clientSet.CoreV1().RESTClient()).Events(namespace)})
	return k.broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: name})
}
