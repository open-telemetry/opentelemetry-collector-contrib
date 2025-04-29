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
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

var _ resolver = (*k8sResolver)(nil)

var (
	errNoSvc = errors.New("no service specified to resolve the backends")

	k8sResolverAttr           = attribute.String("resolver", "k8s")
	k8sResolverAttrSet        = attribute.NewSet(k8sResolverAttr)
	k8sResolverSuccessAttrSet = attribute.NewSet(k8sResolverAttr, attribute.Bool("success", true))
	k8sResolverFailureAttrSet = attribute.NewSet(k8sResolverAttr, attribute.Bool("success", false))
)

const (
	defaultListWatchTimeout = 1 * time.Second
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

	lwTimeout time.Duration

	endpoints         []string
	onChangeCallbacks []func([]string)
	returnNames       bool

	stopCh             chan struct{}
	updateLock         sync.RWMutex
	shutdownWg         sync.WaitGroup
	changeCallbackLock sync.RWMutex

	telemetry *metadata.TelemetryBuilder
}

func newK8sResolver(clt kubernetes.Interface,
	logger *zap.Logger,
	service string,
	ports []int32,
	timeout time.Duration,
	returnNames bool,
	tb *metadata.TelemetryBuilder,
) (*k8sResolver, error) {
	if len(service) == 0 {
		return nil, errNoSvc
	}

	if timeout == 0 {
		timeout = defaultListWatchTimeout
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
			options.TimeoutSeconds = ptr.To[int64](int64(timeout.Seconds()))
			return clt.CoreV1().Endpoints(namespace).List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = epsSelector
			options.TimeoutSeconds = ptr.To[int64](int64(timeout.Seconds()))
			return clt.CoreV1().Endpoints(namespace).Watch(context.Background(), options)
		},
	}

	epsStore := &sync.Map{}
	h := &handler{
		endpoints:   epsStore,
		logger:      logger,
		telemetry:   tb,
		returnNames: returnNames,
	}
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
		lwTimeout:      timeout,
		telemetry:      tb,
		returnNames:    returnNames,
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
		zap.Int32s("ports", r.port),
		zap.Duration("timeout", r.lwTimeout))
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
	var ep string
	r.endpointsStore.Range(func(host, _ any) bool {
		switch r.returnNames {
		case true:
			ep = fmt.Sprintf("%s.%s.%s", host, r.svcName, r.svcNs)
		default:
			ep = host.(string)
		}
		if len(r.port) == 0 {
			backends = append(backends, ep)
		} else {
			for _, port := range r.port {
				backends = append(backends, net.JoinHostPort(ep, strconv.FormatInt(int64(port), 10)))
			}
		}
		return true
	})
	r.telemetry.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(k8sResolverSuccessAttrSet))

	// keep it always in the same order
	sort.Strings(backends)

	if slices.Equal(r.Endpoints(), backends) {
		return r.Endpoints(), nil
	}

	// the list has changed!
	r.updateLock.Lock()
	r.endpoints = backends
	r.updateLock.Unlock()
	r.telemetry.LoadbalancerNumBackends.Record(ctx, int64(len(backends)), metric.WithAttributeSet(k8sResolverAttrSet))
	r.telemetry.LoadbalancerNumBackendUpdates.Add(ctx, 1, metric.WithAttributeSet(k8sResolverAttrSet))

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
		return "", errors.New("not running in-cluster, please specify namespace")
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
