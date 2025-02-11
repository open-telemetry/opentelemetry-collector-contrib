// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
)

const (
	cacheTTL                  = 10 * time.Minute
	OperatingSystemWindows    = "windows"
	RunInContainer            = "RUN_IN_CONTAINER"
	TrueValue                 = "True"
	RunAsHostProcessContainer = "RUN_AS_HOST_PROCESS_CONTAINER"
)

// Option is a struct that can be used to change the configuration of passed K8sClient
// It can be used as an option to the Get(...) function to create a customized K8sClient
type Option struct {
	name string
	set  func(*K8sClient)
}

var mu = &sync.Mutex{}

var optionsToK8sClient = map[string]*K8sClient{}

type stopper interface {
	shutdown()
}

func shutdownClient(client stopper, mu *sync.Mutex, afterShutdown func()) {
	mu.Lock()
	if client != nil {
		client.shutdown()
		afterShutdown()
	}
	mu.Unlock()
}

type cacheReflector interface {
	LastSyncResourceVersion() string
	Run(<-chan struct{})
}

type initialSyncChecker interface {
	// check the initial sync of cache reflector and log the warnMessage if timeout
	Check(reflector cacheReflector, warnMessage string)
}

// reflectorSyncChecker implements initialSyncChecker interface
type reflectorSyncChecker struct {
	pollInterval time.Duration
	pollTimeout  time.Duration
	logger       *zap.Logger
}

func (r *reflectorSyncChecker) Check(reflector cacheReflector, warnMessage string) {
	if err := wait.PollUntilContextTimeout(context.Background(), r.pollInterval, r.pollTimeout, false, func(context.Context) (done bool, err error) {
		return reflector.LastSyncResourceVersion() != "", nil
	}); err != nil {
		r.logger.Warn(warnMessage, zap.Error(err))
	}
}

// KubeConfigPath provides the option to set the kube config which will be used if the
// service account that kubernetes gives to pods can't be used
func KubeConfigPath(kubeConfigPath string) Option {
	return Option{
		name: "kubeConfigPath:" + kubeConfigPath,
		set: func(kc *K8sClient) {
			kc.kubeConfigPath = kubeConfigPath
		},
	}
}

// InitSyncPollInterval provides the option to set the init sync poll interval
// for testing connection to kubernetes api server
func InitSyncPollInterval(pollInterval time.Duration) Option {
	return Option{
		name: "initSyncPollInterval:" + pollInterval.String(),
		set: func(kc *K8sClient) {
			kc.initSyncPollInterval = pollInterval
		},
	}
}

// InitSyncPollTimeout provides the option to set the init sync poll timeout
// for testing connection to kubernetes api server
func InitSyncPollTimeout(pollTimeout time.Duration) Option {
	return Option{
		name: "initSyncPollTimeout:" + pollTimeout.String(),
		set: func(kc *K8sClient) {
			kc.initSyncPollTimeout = pollTimeout
		},
	}
}

// NodeSelector provides the option to provide a field selector
// when retrieving information using the node client
func NodeSelector(nodeSelector fields.Selector) Option {
	return Option{
		name: "nodeSelector:" + nodeSelector.String(),
		set: func(kc *K8sClient) {
			kc.nodeSelector = nodeSelector
		},
	}
}

// CaptureNodeLevelInfo allows one to specify whether node level info
// should be captured and retained in memory
func CaptureNodeLevelInfo(captureNodeLevelInfo bool) Option {
	return Option{
		name: "captureNodeLevelInfo:" + strconv.FormatBool(captureNodeLevelInfo),
		set: func(kc *K8sClient) {
			kc.captureNodeLevelInfo = captureNodeLevelInfo
		},
	}
}

// CaptureOnlyNodeLabelsInfo allows one to specify whether node label
// should be captured and retained in memory
func CaptureOnlyNodeLabelsInfo(captureOnlyNodeLabelInfo bool) Option {
	return Option{
		name: "captureOnlyNodeLabelInfo:" + strconv.FormatBool(captureOnlyNodeLabelInfo),
		set: func(kc *K8sClient) {
			kc.captureOnlyNodeLabelInfo = captureOnlyNodeLabelInfo
		},
	}
}

func getStringifiedOptions(options ...Option) string {
	opts := make([]string, len(options))
	for i, option := range options {
		opts[i] = option.name
	}

	sort.Strings(opts)
	return strings.Join(opts, "+")
}

// Get returns a singleton instance of k8s client
// If the intialization fails, it returns nil
func Get(logger *zap.Logger, options ...Option) *K8sClient {
	strOptions := getStringifiedOptions(options...)

	mu.Lock()
	if optionsToK8sClient[strOptions] == nil {
		// construct the k8s client
		k8sClient := new(K8sClient)
		err := k8sClient.init(logger, options...)
		if err == nil {
			optionsToK8sClient[strOptions] = k8sClient
		}
	}
	mu.Unlock()

	return optionsToK8sClient[strOptions]
}

type epClientWithStopper interface {
	EpClient
	stopper
}

type jobClientWithStopper interface {
	JobClient
	stopper
}

type nodeClientWithStopper interface {
	NodeClient
	stopper
}

type podClientWithStopper interface {
	PodClient
	stopper
}

type replicaSetClientWithStopper interface {
	ReplicaSetClient
	stopper
}

type deploymentClientWithStopper interface {
	DeploymentClient
	stopper
}

type daemonSetClientWithStopper interface {
	DaemonSetClient
	stopper
}

type statefulSetClientWithStopper interface {
	StatefulSetClient
	stopper
}

type K8sClient struct {
	kubeConfigPath       string
	initSyncPollInterval time.Duration
	initSyncPollTimeout  time.Duration

	clientSet kubernetes.Interface

	syncChecker *reflectorSyncChecker

	epMu sync.Mutex
	ep   epClientWithStopper

	podMu sync.Mutex
	pod   podClientWithStopper

	nodeMu sync.Mutex
	node   nodeClientWithStopper

	nodeSelector             fields.Selector
	captureNodeLevelInfo     bool
	captureOnlyNodeLabelInfo bool

	jobMu sync.Mutex
	job   jobClientWithStopper

	rsMu       sync.Mutex
	replicaSet replicaSetClientWithStopper

	dMu        sync.Mutex
	deployment deploymentClientWithStopper

	dsMu      sync.Mutex
	daemonSet daemonSetClientWithStopper

	ssMu        sync.Mutex
	statefulSet statefulSetClientWithStopper

	logger *zap.Logger
}

func (c *K8sClient) init(logger *zap.Logger, options ...Option) error {
	c.logger = logger

	// set up some default configs
	c.kubeConfigPath = filepath.Join(os.Getenv("HOME"), ".kube/config")
	c.initSyncPollInterval = 50 * time.Millisecond
	c.initSyncPollTimeout = 2 * time.Second

	// take additional options passed in
	for _, opt := range options {
		opt.set(c)
	}

	config, err := c.inClusterConfig()
	if err != nil {
		c.logger.Warn("cannot find in cluster config", zap.Error(err))
		config, err = clientcmd.BuildConfigFromFlags("", c.kubeConfigPath)
		if err != nil {
			c.logger.Error("failed to build config", zap.Error(err))
			return err
		}
	}
	config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	config.ContentType = "application/vnd.kubernetes.protobuf"
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		c.logger.Error("failed to build ClientSet", zap.Error(err))
		return err
	}

	c.syncChecker = &reflectorSyncChecker{
		pollInterval: c.initSyncPollInterval,
		pollTimeout:  c.initSyncPollTimeout,
		logger:       c.logger,
	}

	c.clientSet = client
	c.ep = nil
	c.pod = nil
	c.node = nil
	c.job = nil
	c.replicaSet = nil
	c.deployment = nil
	c.daemonSet = nil
	c.statefulSet = nil

	return nil
}

func (c *K8sClient) GetEpClient() EpClient {
	c.epMu.Lock()
	if c.ep == nil {
		c.ep = newEpClient(c.clientSet, c.logger, epSyncCheckerOption(c.syncChecker))
	}
	c.epMu.Unlock()
	return c.ep
}

func (c *K8sClient) ShutdownEpClient() {
	shutdownClient(c.ep, &c.epMu, func() {
		c.ep = nil
	})
}

func (c *K8sClient) GetPodClient() PodClient {
	c.podMu.Lock()
	if c.pod == nil {
		c.pod = newPodClient(c.clientSet, c.logger, podSyncCheckerOption(c.syncChecker))
	}
	c.podMu.Unlock()
	return c.pod
}

func (c *K8sClient) ShutdownPodClient() {
	shutdownClient(c.pod, &c.podMu, func() {
		c.pod = nil
	})
}

func (c *K8sClient) GetNodeClient() NodeClient {
	c.nodeMu.Lock()
	if c.node == nil {
		opts := []nodeClientOption{
			nodeSyncCheckerOption(c.syncChecker), captureNodeLevelInfoOption(c.captureNodeLevelInfo),
			captureOnlyNodeLabelInfoOption(c.captureOnlyNodeLabelInfo),
		}
		if c.nodeSelector != nil {
			opts = append(opts, nodeSelectorOption(c.nodeSelector))
		}
		c.node = newNodeClient(c.clientSet, c.logger, opts...)
	}
	c.nodeMu.Unlock()
	return c.node
}

func (c *K8sClient) ShutdownNodeClient() {
	shutdownClient(c.node, &c.nodeMu, func() {
		c.node = nil
	})
}

func (c *K8sClient) GetJobClient() JobClient {
	var err error
	c.jobMu.Lock()
	if c.job == nil {
		c.job, err = newJobClient(c.clientSet, c.logger, jobSyncCheckerOption(c.syncChecker))
		if err != nil {
			c.logger.Error("use an no-op job client instead because of error", zap.Error(err))
			c.job = &noOpJobClient{}
		}
	}
	c.jobMu.Unlock()
	return c.job
}

func (c *K8sClient) ShutdownJobClient() {
	shutdownClient(c.job, &c.jobMu, func() {
		c.job = nil
	})
}

func (c *K8sClient) GetReplicaSetClient() ReplicaSetClient {
	var err error
	c.rsMu.Lock()
	if c.replicaSet == nil || reflect.ValueOf(c.replicaSet).IsNil() {
		c.replicaSet, err = newReplicaSetClient(c.clientSet, c.logger, replicaSetSyncCheckerOption(c.syncChecker))
		if err != nil {
			c.logger.Error("use an no-op replica set client instead because of error", zap.Error(err))
			c.replicaSet = &noOpReplicaSetClient{}
		}
	}
	c.rsMu.Unlock()
	return c.replicaSet
}

func (c *K8sClient) ShutdownReplicaSetClient() {
	shutdownClient(c.replicaSet, &c.rsMu, func() {
		c.replicaSet = nil
	})
}

func (c *K8sClient) GetDeploymentClient() DeploymentClient {
	var err error
	c.dMu.Lock()
	if c.deployment == nil || reflect.ValueOf(c.deployment).IsNil() {
		c.deployment, err = newDeploymentClient(c.clientSet, c.logger, deploymentSyncCheckerOption(c.syncChecker))
		if err != nil {
			c.logger.Error("use an no-op deployment client instead because of error", zap.Error(err))
			c.deployment = &noOpDeploymentClient{}
		}
	}
	c.dMu.Unlock()
	return c.deployment
}

func (c *K8sClient) ShutdownDeploymentClient() {
	shutdownClient(c.deployment, &c.dMu, func() {
		c.deployment = nil
	})
}

func (c *K8sClient) GetDaemonSetClient() DaemonSetClient {
	var err error
	c.dsMu.Lock()
	if c.daemonSet == nil || reflect.ValueOf(c.daemonSet).IsNil() {
		c.daemonSet, err = newDaemonSetClient(c.clientSet, c.logger, daemonSetSyncCheckerOption(c.syncChecker))
		if err != nil {
			c.logger.Error("use an no-op daemonSet client instead because of error", zap.Error(err))
			c.daemonSet = &noOpDaemonSetClient{}
		}
	}
	c.dsMu.Unlock()
	return c.daemonSet
}

func (c *K8sClient) ShutdownDaemonSetClient() {
	shutdownClient(c.daemonSet, &c.dsMu, func() {
		c.daemonSet = nil
	})
}

func (c *K8sClient) GetStatefulSetClient() StatefulSetClient {
	var err error
	c.ssMu.Lock()
	defer c.ssMu.Unlock()
	if c.statefulSet == nil || reflect.ValueOf(c.statefulSet).IsNil() {
		c.statefulSet, err = newStatefulSetClient(c.clientSet, c.logger, statefulSetSyncCheckerOption(c.syncChecker))
		if err != nil {
			c.logger.Error("use an no-op statefulSet client instead because of error", zap.Error(err))
			c.statefulSet = &noOpStatefulSetClient{}
		}
	}
	return c.statefulSet
}

func (c *K8sClient) ShutdownStatefulSetClient() {
	shutdownClient(c.statefulSet, &c.ssMu, func() {
		c.statefulSet = nil
	})
}

func (c *K8sClient) GetClientSet() kubernetes.Interface {
	return c.clientSet
}

// Shutdown stops K8sClient
func (c *K8sClient) Shutdown() {
	mu.Lock()
	defer mu.Unlock()

	c.ShutdownEpClient()
	c.ShutdownPodClient()
	c.ShutdownNodeClient()
	c.ShutdownJobClient()
	c.ShutdownReplicaSetClient()
	c.ShutdownDeploymentClient()
	c.ShutdownDaemonSetClient()
	c.ShutdownStatefulSetClient()

	// remove the current instance of k8s client from map
	for key, val := range optionsToK8sClient {
		if val == c {
			delete(optionsToK8sClient, key)
		}
	}
}

// inClusterConfig is copy of rest.InClusterConfig.
// There is known bug in rest.InClusterConfig on Windows when running it as host process container.
// https://github.com/kubernetes/kubernetes/issues/104562
// This copy fixes that bug by appending `CONTAINER_SANDBOX_MOUNT_POINT` in k8s token and cert file paths.
// todo: Remove this workaround func when Windows AMIs has containerd 1.7 which solves upstream bug.
func (c *K8sClient) inClusterConfig() (*rest.Config, error) {
	if !isWindowsHostProcessContainer() {
		return rest.InClusterConfig()
	}
	var (
		tokenFile  = filepath.Join(os.Getenv("CONTAINER_SANDBOX_MOUNT_POINT"), "/var/run/secrets/kubernetes.io/serviceaccount/token")
		rootCAFile = filepath.Join(os.Getenv("CONTAINER_SANDBOX_MOUNT_POINT"), "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	)
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, rest.ErrNotInCluster
	}

	token, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}

	tlsClientConfig := rest.TLSClientConfig{}

	if _, err := certutil.NewPool(rootCAFile); err != nil {
		c.logger.Error(fmt.Sprintf("Expected to load root CA config from %s, but got err: %v", rootCAFile, err))
	} else {
		tlsClientConfig.CAFile = rootCAFile
	}

	return &rest.Config{
		Host:            "https://" + net.JoinHostPort(host, port),
		TLSClientConfig: tlsClientConfig,
		BearerToken:     string(token),
		BearerTokenFile: tokenFile,
	}, nil
}

func isWindowsHostProcessContainer() bool {
	// todo: Remove this workaround func when Windows AMIs has containerd 1.7 which solves upstream bug
	// https://kubernetes.io/docs/tasks/configure-pod-container/create-hostprocess-pod/#containerd-v1-6
	if runtime.GOOS == OperatingSystemWindows && os.Getenv(RunInContainer) == TrueValue && os.Getenv(RunAsHostProcessContainer) == TrueValue {
		return true
	}
	return false
}
