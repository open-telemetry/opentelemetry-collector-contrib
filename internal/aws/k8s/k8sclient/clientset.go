// Copyright  OpenTelemetry Authors
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

package k8sclient

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	cacheTTL = 10 * time.Minute
)

// Option is a struct that can be used to change the configuration of passed K8sClient
// It can be used as an option to the Get(...) function to create a customized K8sClient
type Option struct {
	name string
	set  func(*K8sClient)
}

var mu = &sync.Mutex{}

var optionsToK8sClient = map[string]*K8sClient{}

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
	if err := wait.Poll(r.pollInterval, r.pollTimeout, func() (done bool, err error) {
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

func getStringifiedOptions(options ...Option) string {
	opts := make([]string, 0)
	for _, option := range options {
		opts = append(opts, option.name)
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
		//construct the k8s client
		k8sClient := new(K8sClient)
		err := k8sClient.init(logger, options...)
		if err == nil {
			optionsToK8sClient[strOptions] = k8sClient
		}
	}
	mu.Unlock()

	return optionsToK8sClient[strOptions]
}

type K8sClient struct {
	kubeConfigPath       string
	initSyncPollInterval time.Duration
	initSyncPollTimeout  time.Duration

	ClientSet kubernetes.Interface

	Ep   EpClient
	Pod  PodClient
	Node NodeClient

	Job        JobClient
	ReplicaSet ReplicaSetClient

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

	config, err := rest.InClusterConfig()
	if err != nil {
		c.logger.Warn("cannot find in cluster config", zap.Error(err))
		config, err = clientcmd.BuildConfigFromFlags("", c.kubeConfigPath)
		if err != nil {
			c.logger.Error("failed to build config", zap.Error(err))
			return err
		}
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		c.logger.Error("failed to build ClientSet", zap.Error(err))
		return err
	}

	syncChecker := &reflectorSyncChecker{
		pollInterval: c.initSyncPollInterval,
		pollTimeout:  c.initSyncPollTimeout,
		logger:       c.logger,
	}

	c.ClientSet = client
	c.Ep = newEpClient(client, c.logger, epSyncCheckerOption(syncChecker))
	c.Pod = newPodClient(client, c.logger, podSyncCheckerOption(syncChecker))
	c.Node = newNodeClient(client, c.logger, nodeSyncCheckerOption(syncChecker))

	c.Job, err = newJobClient(client, c.logger, jobSyncCheckerOption(syncChecker))
	if err != nil {
		c.logger.Error("use an no-op job client instead because of error", zap.Error(err))
		c.Job = &noOpJobClient{}
	}
	c.ReplicaSet, err = newReplicaSetClient(client, c.logger, replicaSetSyncCheckerOption(syncChecker))
	if err != nil {
		c.logger.Error("use an no-op replica set client instead because of error", zap.Error(err))
		c.ReplicaSet = &noOpReplicaSetClient{}
	}

	return nil
}

func (c *K8sClient) Shutdown() {
	mu.Lock()
	defer mu.Unlock()

	if c.Ep != nil && !reflect.ValueOf(c.Ep).IsNil() {
		c.Ep.shutdown()
		c.Ep = nil
	}
	if c.Pod != nil && !reflect.ValueOf(c.Pod).IsNil() {
		c.Pod.Shutdown()
		c.Pod = nil
	}
	if c.Node != nil && !reflect.ValueOf(c.Node).IsNil() {
		c.Node.Shutdown()
		c.Node = nil
	}
	if c.Job != nil && !reflect.ValueOf(c.Job).IsNil() {
		c.Job.shutdown()
		c.Job = nil
	}
	if c.ReplicaSet != nil && !reflect.ValueOf(c.ReplicaSet).IsNil() {
		c.ReplicaSet.shutdown()
		c.ReplicaSet = nil
	}

	// remove the current instance of k8s client from map
	for key, val := range optionsToK8sClient {
		if val == c {
			delete(optionsToK8sClient, key)
		}
	}
}
