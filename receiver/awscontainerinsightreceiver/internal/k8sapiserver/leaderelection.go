package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
)

type LeaderElection struct {
	logger   *zap.Logger
	cancel   context.CancelFunc
	nodeName string

	mu                           sync.Mutex
	leading                      bool
	leaderLockName               string
	leaderLockUsingConfigMapOnly bool

	k8sClient        K8sClient // *k8sclient.K8sClient
	epClient         k8sclient.EpClient
	nodeClient       k8sclient.NodeClient
	podClient        k8sclient.PodClient
	deploymentClient k8sclient.DeploymentClient
	daemonSetClient  k8sclient.DaemonSetClient

	// the following can be set to mocks in testing
	broadcaster eventBroadcaster
	// the close of isLeadingC indicates the leader election is done. This is used in testing
	isLeadingC chan bool
}

type eventBroadcaster interface {
	// StartRecordingToSink starts sending events received from this EventBroadcaster to the given
	// sink. The return value can be ignored or used to stop recording, if desired.
	StartRecordingToSink(sink record.EventSink) watch.Interface
	// StartLogging starts sending events received from this EventBroadcaster to the given logging
	// function. The return value can be ignored or used to stop recording, if desired.
	StartLogging(logf func(format string, args ...interface{})) watch.Interface
	// NewRecorder returns an EventRecorder that can be used to send events to this EventBroadcaster
	// with the event source set to the given event source.
	NewRecorder(scheme *runtime.Scheme, source v1.EventSource) record.EventRecorder
}

type K8sClient interface {
	GetClientSet() kubernetes.Interface
	GetEpClient() k8sclient.EpClient
	GetNodeClient() k8sclient.NodeClient
	GetPodClient() k8sclient.PodClient
	GetDeploymentClient() k8sclient.DeploymentClient
	GetDaemonSetClient() k8sclient.DaemonSetClient
	ShutdownNodeClient()
	ShutdownPodClient()
}

type LeaderElectionOption func(*LeaderElection)

func WithLeaderLockName(name string) LeaderElectionOption {
	return func(le *LeaderElection) {
		le.leaderLockName = name
	}
}

func WithLeaderLockUsingConfigMapOnly(leaderLockUsingConfigMapOnly bool) LeaderElectionOption {
	return func(le *LeaderElection) {
		le.leaderLockUsingConfigMapOnly = leaderLockUsingConfigMapOnly
	}
}

func NewLeaderElection(logger *zap.Logger, options ...LeaderElectionOption) (*LeaderElection, error) {
	le := &LeaderElection{
		logger:      logger,
		k8sClient:   k8sclient.Get(logger),
		broadcaster: record.NewBroadcaster(),
	}

	for _, opt := range options {
		opt(le)
	}

	if le.k8sClient == nil {
		return nil, errors.New("failed to perform leaderelection because k8sclient is nil")
	}

	if err := le.init(); err != nil {
		return nil, err
	}

	return le, nil
}

func (le *LeaderElection) init() error {
	var ctx context.Context
	ctx, le.cancel = context.WithCancel(context.Background())

	le.nodeName = os.Getenv("HOST_NAME")
	if le.nodeName == "" {
		return errors.New("environment variable HOST_NAME is not set in k8s deployment config")
	}

	lockNamespace := os.Getenv("K8S_NAMESPACE")
	if lockNamespace == "" {
		return errors.New("environment variable K8S_NAMESPACE is not set in k8s deployment config")
	}

	resourceLockConfig := resourcelock.ResourceLockConfig{
		Identity:      le.nodeName,
		EventRecorder: le.createRecorder(le.leaderLockName, lockNamespace),
	}

	clientSet := le.k8sClient.GetClientSet()
	configMapInterface := clientSet.CoreV1().ConfigMaps(lockNamespace)
	configMap, err := configMapInterface.Get(ctx, le.leaderLockName, metav1.GetOptions{})
	if configMap == nil || err != nil {
		le.logger.Info(fmt.Sprintf("Cannot get the leader config map: %v, try to create the config map...", err))
		configMap, err = configMapInterface.Create(ctx,
			&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: lockNamespace,
					Name:      le.leaderLockName,
				},
			}, metav1.CreateOptions{})
		le.logger.Info(fmt.Sprintf("configMap: %v, err: %v", configMap, err))
	}

	le.logger.Info(fmt.Sprintf("configMap: %v, err: %v", configMap, err))

	var lock resourcelock.Interface
	if le.leaderLockUsingConfigMapOnly {
		lock = &ConfigMapLock{
			ConfigMapMeta: metav1.ObjectMeta{
				Namespace: lockNamespace,
				Name:      le.leaderLockName,
			},
			Client:     clientSet.CoreV1(),
			LockConfig: resourceLockConfig,
		}
	} else {
		l, err := resourcelock.New(
			resourcelock.ConfigMapsLeasesResourceLock,
			lockNamespace, le.leaderLockName,
			clientSet.CoreV1(),
			clientSet.CoordinationV1(),
			resourceLockConfig)
		if err != nil {
			le.logger.Warn("Failed to create resource lock", zap.Error(err))
			return err
		}
		lock = l
	}

	go le.startLeaderElection(ctx, lock)

	return nil
}

func (le *LeaderElection) startLeaderElection(ctx context.Context, lock resourcelock.Interface) {

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
					le.logger.Info(fmt.Sprintf("OnStartedLeading: %s", le.nodeName))
					// we're notified when we start
					le.mu.Lock()
					le.leading = true
					// always retrieve clients in case previous ones shut down during leader switching
					le.nodeClient = le.k8sClient.GetNodeClient()
					le.podClient = le.k8sClient.GetPodClient()
					le.epClient = le.k8sClient.GetEpClient()
					le.deploymentClient = le.k8sClient.GetDeploymentClient()
					le.daemonSetClient = le.k8sClient.GetDaemonSetClient()
					le.mu.Unlock()

					if le.isLeadingC != nil {
						// this executes only in testing
						close(le.isLeadingC)
					}

					for {
						le.mu.Lock()
						leading := le.leading
						le.mu.Unlock()
						if !leading {
							le.logger.Info("no longer leading")
							return
						}
						select {
						case <-ctx.Done():
							le.logger.Info("ctx cancelled")
							return
						case <-time.After(time.Second):
						}
					}
				},
				OnStoppedLeading: func() {
					le.logger.Info(fmt.Sprintf("OnStoppedLeading: %s", le.nodeName))
					// we can do cleanup here, or after the RunOrDie method returns
					le.mu.Lock()
					defer le.mu.Unlock()
					le.leading = false
					// node and pod are only used for cluster level metrics, endpoint is used for decorator too.
					le.k8sClient.ShutdownNodeClient()
					le.k8sClient.ShutdownPodClient()
				},
				OnNewLeader: func(identity string) {
					le.logger.Info(fmt.Sprintf("Switch NewLeaderElection Leader: %s", identity))
				},
			},
		})

		select {
		case <-ctx.Done(): // when leader election ends, the channel ctx.Done() will be closed
			le.logger.Info(fmt.Sprintf("shutdown Leader Election: %s", le.nodeName))
			return
		default:
		}
	}
}

func (le *LeaderElection) createRecorder(name, namespace string) record.EventRecorder {
	le.broadcaster.StartLogging(klog.Infof)
	clientSet := le.k8sClient.GetClientSet()
	le.broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: corev1.New(clientSet.CoreV1().RESTClient()).Events(namespace)})
	return le.broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: name})
}
