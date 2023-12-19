// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ resolver = (*k8sResolver)(nil)

var (
	errNoSvc                        = errors.New("no service specified to resolve the backends")
	k8sResolverMutator              = tag.Upsert(tag.MustNewKey("resolver"), "k8s")
	k8sResolverSuccessTrueMutators  = []tag.Mutator{k8sResolverMutator, successTrueMutator}
	k8sResolverSuccessFalseMutators = []tag.Mutator{k8sResolverMutator, successFalseMutator}
)

type k8sResolver struct {
	logger  *zap.Logger
	svcName string
	svcNs   string
	port    []int32

	handler        *handler
	once           *sync.Once
	epsListWatcher cache.ListerWatcher
	endpointsStore *sync.Map

	endpoints         []string
	onChangeCallbacks []func([]string)

	stopCh             chan struct{}
	updateLock         sync.RWMutex
	shutdownWg         sync.WaitGroup
	changeCallbackLock sync.RWMutex
}

func newK8sResolver(clt kubernetes.Interface,
	logger *zap.Logger,
	service string,
	ports []int32) (*k8sResolver, error) {

	if len(service) == 0 {
		return nil, errNoSvc
	}

	nAddr := strings.SplitN(service, ".", 2)
	name, namespace := nAddr[0], "default"
	if len(nAddr) > 1 {
		namespace = nAddr[1]
	} else {
		logger.Info("the namespace for the Kubernetes service wasn't provided, trying to determine the current namespace", zap.String("name", name))
		if ns, err := getInClusterNamespace(); err == nil {
			namespace = ns
			logger.Info("namespace for the Collector determined", zap.String("namespace", namespace))
		} else {
			logger.Warn(`could not determine the namespace for this collector, will use "default" as the namespace`, zap.Error(err))
		}
	}

	epsSelector := fmt.Sprintf("metadata.name=%s", name)
	epsListWatcher := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = epsSelector
			options.TimeoutSeconds = ptr.To[int64](1)
			return clt.CoreV1().Endpoints(namespace).List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = epsSelector
			options.TimeoutSeconds = ptr.To[int64](1)
			return clt.CoreV1().Endpoints(namespace).Watch(context.Background(), options)
		},
	}

	epsStore := &sync.Map{}
	h := &handler{endpoints: epsStore, logger: logger}
	r := &k8sResolver{
		logger:         logger,
		svcName:        name,
		svcNs:          namespace,
		port:           ports,
		once:           &sync.Once{},
		endpointsStore: epsStore,
		epsListWatcher: epsListWatcher,
		handler:        h,
		stopCh:         make(chan struct{}),
	}
	h.callback = r.resolve

	return r, nil
}

func (r *k8sResolver) start(_ context.Context) error {
	var initErr error
	r.once.Do(func() {
		if r.epsListWatcher != nil {
			r.logger.Debug("creating and starting endpoints informer")
			epsInformer := cache.NewSharedInformer(r.epsListWatcher, &corev1.Endpoints{}, 0)
			if _, err := epsInformer.AddEventHandler(r.handler); err != nil {
				r.logger.Error("unable to start watching for changes to the specified service names", zap.Error(err))
			}
			go epsInformer.Run(r.stopCh)
			if !cache.WaitForCacheSync(r.stopCh, epsInformer.HasSynced) {
				initErr = errors.New("endpoints informer not sync")
			}
		}
	})
	if initErr != nil {
		return initErr
	}

	r.logger.Debug("K8s service resolver started",
		zap.String("service", r.svcName),
		zap.String("namespace", r.svcNs),
		zap.Int32s("ports", r.port))
	return nil
}

func (r *k8sResolver) shutdown(_ context.Context) error {
	r.changeCallbackLock.Lock()
	r.onChangeCallbacks = nil
	r.changeCallbackLock.Unlock()

	close(r.stopCh)
	r.shutdownWg.Wait()
	return nil
}
func newInClusterClient() (kubernetes.Interface, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func (r *k8sResolver) resolve(ctx context.Context) ([]string, error) {
	r.shutdownWg.Add(1)
	defer r.shutdownWg.Done()

	var backends []string
	r.endpointsStore.Range(func(address, value any) bool {
		addr := address.(string)
		if len(r.port) == 0 {
			backends = append(backends, addr)
		} else {
			for _, port := range r.port {
				backends = append(backends, net.JoinHostPort(addr, strconv.FormatInt(int64(port), 10)))
			}
		}
		return true
	})
	_ = stats.RecordWithTags(ctx, k8sResolverSuccessTrueMutators, mNumResolutions.M(1))

	// keep it always in the same order
	sort.Strings(backends)

	if slices.Equal(r.Endpoints(), backends) {
		return r.Endpoints(), nil
	}

	// the list has changed!
	r.updateLock.Lock()
	r.endpoints = backends
	r.updateLock.Unlock()
	_ = stats.RecordWithTags(ctx, k8sResolverSuccessTrueMutators, mNumBackends.M(int64(len(backends))))

	// propagate the change
	r.changeCallbackLock.RLock()
	for _, callback := range r.onChangeCallbacks {
		callback(r.Endpoints())
	}
	r.changeCallbackLock.RUnlock()
	return r.Endpoints(), nil
}

func (r *k8sResolver) onChange(f func([]string)) {
	r.changeCallbackLock.Lock()
	defer r.changeCallbackLock.Unlock()
	r.onChangeCallbacks = append(r.onChangeCallbacks, f)
}
func (r *k8sResolver) Endpoints() []string {
	r.updateLock.RLock()
	defer r.updateLock.RUnlock()
	return r.endpoints
}

const inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func getInClusterNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	if _, err := os.Stat(inClusterNamespacePath); os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, please specify namespace")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	// Load the namespace file and return its content
	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}
