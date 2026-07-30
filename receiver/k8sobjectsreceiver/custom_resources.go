// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/filter"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
)

const (
	customResourcePageSize         int64 = 100
	customResourceListMaxAttempts        = 3
	customResourceListRetryDelay         = time.Second
	customResourceCacheSyncTimeout       = 2 * time.Minute
)

var customResourceDefinitionGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

type discoveredCustomResource struct {
	gvr        schema.GroupVersionResource
	namespaced bool
}

type customResourceCollector struct {
	config           *CustomResourcesConfig
	dynamicClient    dynamic.Interface
	discoveryClient  discovery.ServerResourcesInterface
	logger           *zap.Logger
	namespaceFilter  filter.Filter
	skipResources    map[schema.GroupResource]struct{}
	consume          func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource)
	crdInformer      cache.SharedIndexInformer
	informerReady    <-chan error
	discoveryDirty   atomic.Bool
	discovered       []discoveredCustomResource
	listRetryDelay   time.Duration
	cacheSyncTimeout time.Duration
}

func newCustomResourceCollector(
	config *CustomResourcesConfig,
	dynamicClient dynamic.Interface,
	metadataClient metadata.Interface,
	discoveryClient discovery.ServerResourcesInterface,
	skipResources map[schema.GroupResource]struct{},
	logger *zap.Logger,
	consume func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource),
) (*customResourceCollector, error) {
	crdInformer, informerReady := newCustomResourceDefinitionInformer(metadataClient)
	collector := &customResourceCollector{
		config:           config,
		dynamicClient:    dynamicClient,
		discoveryClient:  discoveryClient,
		logger:           logger,
		namespaceFilter:  filter.CreateFilter(config.ExcludeNamespaces),
		skipResources:    skipResources,
		consume:          consume,
		crdInformer:      crdInformer,
		informerReady:    informerReady,
		listRetryDelay:   customResourceListRetryDelay,
		cacheSyncTimeout: customResourceCacheSyncTimeout,
	}
	collector.discoveryDirty.Store(true)
	_, err := crdInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(any) {
			collector.discoveryDirty.Store(true)
		},
		UpdateFunc: func(any, any) {
			collector.discoveryDirty.Store(true)
		},
		DeleteFunc: func(any) {
			collector.discoveryDirty.Store(true)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register custom resource definition event handler: %w", err)
	}
	return collector, nil
}

func newCustomResourceDefinitionInformer(metadataClient metadata.Interface) (cache.SharedIndexInformer, <-chan error) {
	resource := metadataClient.Resource(customResourceDefinitionGVR).Namespace(metav1.NamespaceAll)
	informerReady := make(chan error, 1)
	var informerReadyOnce sync.Once
	reportReady := func(err error) {
		informerReadyOnce.Do(func() {
			informerReady <- err
		})
	}
	reportAuthorizationError := func(operation string, err error) {
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
			reportReady(fmt.Errorf("failed to %s custom resource definitions: %w", operation, err))
		}
	}
	listWatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			result, err := resource.List(context.Background(), options)
			reportAuthorizationError("list", err)
			return result, err
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			result, err := resource.Watch(context.Background(), options)
			if err == nil {
				reportReady(nil)
			} else {
				reportAuthorizationError("watch", err)
			}
			return result, err
		},
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			result, err := resource.List(ctx, options)
			reportAuthorizationError("list", err)
			return result, err
		},
		WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			result, err := resource.Watch(ctx, options)
			if err == nil {
				reportReady(nil)
			} else {
				reportAuthorizationError("watch", err)
			}
			return result, err
		},
	}
	return cache.NewSharedIndexInformer(
		cache.ToListWatcherWithWatchListSemantics(listWatch, metadataClient),
		&metav1.PartialObjectMetadata{},
		0,
		cache.Indexers{},
	), informerReady
}

func (c *customResourceCollector) Start(ctx context.Context, wg *sync.WaitGroup) (chan struct{}, error) {
	stop := make(chan struct{})
	collectorCtx, cancel := context.WithCancel(ctx)
	wg.Go(func() {
		select {
		case <-stop:
			cancel()
		case <-collectorCtx.Done():
		}
	})
	wg.Go(func() {
		c.crdInformer.RunWithContext(collectorCtx)
	})

	if err := c.waitForInformer(collectorCtx); err != nil {
		cancel()
		return nil, err
	}

	wg.Go(func() {
		if c.config.InitialDelay > 0 {
			timer := time.NewTimer(c.config.InitialDelay)
			select {
			case <-timer.C:
			case <-collectorCtx.Done():
				timer.Stop()
				return
			}
		}

		for {
			select {
			case <-collectorCtx.Done():
				return
			default:
			}

			if err := c.collect(collectorCtx); err != nil && !errors.Is(err, context.Canceled) {
				c.logger.Error("failed to collect custom resources", zap.Error(err))
			}

			timer := time.NewTimer(c.config.Interval)
			select {
			case <-timer.C:
			case <-collectorCtx.Done():
				timer.Stop()
				return
			}
		}
	})
	return stop, nil
}

func (c *customResourceCollector) waitForInformer(ctx context.Context) error {
	syncCtx, syncCancel := context.WithTimeout(ctx, c.cacheSyncTimeout)
	defer syncCancel()
	cacheSyncResult := make(chan bool, 1)
	go func() {
		cacheSyncResult <- cache.WaitForCacheSync(syncCtx.Done(), c.crdInformer.HasSynced)
	}()

	cacheSynced := false
	watchStarted := false
	for !cacheSynced || !watchStarted {
		select {
		case synced := <-cacheSyncResult:
			if !synced {
				return c.informerStartupError(syncCtx.Err())
			}
			cacheSynced = true
		case err := <-c.informerReady:
			if err != nil {
				return err
			}
			watchStarted = true
		case <-syncCtx.Done():
			return c.informerStartupError(syncCtx.Err())
		}
	}
	return nil
}

func (c *customResourceCollector) informerStartupError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf(
			"timed out after %s waiting for custom resource definition cache synchronization and watch startup",
			c.cacheSyncTimeout,
		)
	}
	if err == nil {
		return errors.New("failed to synchronize custom resource definition cache")
	}
	return fmt.Errorf("failed to initialize custom resource definition informer: %w", err)
}

func (c *customResourceCollector) collect(ctx context.Context) error {
	if err := c.refreshDiscovery(); err != nil {
		return err
	}
	for _, resource := range c.discovered {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.collectResource(ctx, resource); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			c.logger.Error(
				"failed to collect custom resource",
				zap.String("resource", resource.gvr.String()),
				zap.Error(err),
			)
		}
	}
	return nil
}

func (c *customResourceCollector) refreshDiscovery() error {
	if !c.discoveryDirty.Swap(false) {
		return nil
	}

	resources, discoveryErr := c.discoveryClient.ServerPreferredResources()
	if discoveryErr != nil && len(resources) == 0 {
		c.discoveryDirty.Store(true)
		return fmt.Errorf("failed to discover preferred Kubernetes resources: %w", discoveryErr)
	}
	if discoveryErr != nil {
		c.logger.Warn("some Kubernetes API groups could not be discovered", zap.Error(discoveryErr))
		c.discoveryDirty.Store(true)
	}

	result := make(map[schema.GroupResource]struct{})
	for _, object := range c.crdInformer.GetStore().List() {
		crd, ok := object.(*metav1.PartialObjectMetadata)
		if !ok {
			continue
		}
		name, group, ok := strings.Cut(crd.Name, ".")
		if ok {
			result[schema.GroupResource{Group: group, Resource: name}] = struct{}{}
		}
	}

	c.discovered = discoverCustomResources(
		result,
		resources,
		c.config.Include,
		c.config.Exclude,
		c.skipResources,
	)
	c.logger.Debug("discovered custom resources", zap.Int("count", len(c.discovered)))
	return nil
}

func discoverCustomResources(
	crds map[schema.GroupResource]struct{},
	resourceLists []*metav1.APIResourceList,
	include, exclude []CustomResourceSelector,
	skip map[schema.GroupResource]struct{},
) []discoveredCustomResource {
	byGroupResource := make(map[schema.GroupResource]discoveredCustomResource)

	for _, resourceList := range resourceLists {
		groupVersion, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			continue
		}
		for i := range resourceList.APIResources {
			apiResource := &resourceList.APIResources[i]
			groupResource := schema.GroupResource{
				Group:    groupVersion.Group,
				Resource: apiResource.Name,
			}
			if _, ok := crds[groupResource]; !ok || !slices.Contains(apiResource.Verbs, "list") {
				continue
			}
			if _, ok := skip[groupResource]; ok {
				continue
			}
			if !matchesCustomResourceSelectors(groupResource, include) ||
				matchesCustomResourceSelectors(groupResource, exclude) {
				continue
			}
			byGroupResource[groupResource] = discoveredCustomResource{
				gvr:        groupResource.WithVersion(groupVersion.Version),
				namespaced: apiResource.Namespaced,
			}
		}
	}

	result := make([]discoveredCustomResource, 0, len(byGroupResource))
	for _, resource := range byGroupResource {
		result = append(result, resource)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].gvr.Group != result[j].gvr.Group {
			return result[i].gvr.Group < result[j].gvr.Group
		}
		return result[i].gvr.Resource < result[j].gvr.Resource
	})
	return result
}

func matchesCustomResourceSelectors(
	resource schema.GroupResource,
	selectors []CustomResourceSelector,
) bool {
	for _, selector := range selectors {
		if (selector.Group == "*" || selector.Group == resource.Group) &&
			(slices.Contains(selector.Resources, "*") || slices.Contains(selector.Resources, resource.Resource)) {
			return true
		}
	}
	return false
}

func (c *customResourceCollector) collectResource(ctx context.Context, resource discoveredCustomResource) error {
	resourceClient := c.dynamicClient.Resource(resource.gvr)
	if !resource.namespaced || len(c.config.Namespaces) == 0 {
		return c.listResource(ctx, resourceClient, resource.gvr, resource.namespaced)
	}

	var errs []error
	for _, namespace := range c.config.Namespaces {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.listResource(ctx, resourceClient.Namespace(namespace), resource.gvr, false); err != nil {
			errs = append(errs, fmt.Errorf("namespace %q: %w", namespace, err))
		}
	}
	return errors.Join(errs...)
}

func (c *customResourceCollector) listResource(
	ctx context.Context,
	resource dynamic.ResourceInterface,
	gvr schema.GroupVersionResource,
	applyNamespaceFilter bool,
) error {
	options := metav1.ListOptions{
		LabelSelector: c.config.LabelSelector,
		FieldSelector: c.config.FieldSelector,
		Limit:         customResourcePageSize,
	}

	for {
		objects, err := c.listPage(ctx, resource, options)
		if err != nil {
			return err
		}
		if applyNamespaceFilter && len(c.config.ExcludeNamespaces) != 0 {
			filtered := objects.Items[:0]
			for i := range objects.Items {
				if !c.namespaceFilter.Matches(objects.Items[i].GetNamespace()) {
					filtered = append(filtered, objects.Items[i])
				}
			}
			objects.Items = filtered
		}
		if len(objects.Items) != 0 {
			c.consume(ctx, objects, gvr)
		}
		if objects.GetContinue() == "" {
			return nil
		}
		options.Continue = objects.GetContinue()
	}
}

func (c *customResourceCollector) listPage(
	ctx context.Context,
	resource dynamic.ResourceInterface,
	options metav1.ListOptions,
) (*unstructured.UnstructuredList, error) {
	var lastErr error
	for attempt := range customResourceListMaxAttempts {
		objects, err := resource.List(ctx, options)
		if err == nil {
			return objects, nil
		}
		lastErr = err

		delay, retry := customResourceListErrorRetryDelay(err, c.listRetryDelay)
		if !retry || attempt == customResourceListMaxAttempts-1 {
			return nil, err
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func customResourceListErrorRetryDelay(err error, fallback time.Duration) (time.Duration, bool) {
	if seconds, ok := apierrors.SuggestsClientDelay(err); ok && seconds > 0 {
		return time.Duration(seconds) * time.Second, true
	}
	if apierrors.IsTooManyRequests(err) ||
		apierrors.IsServiceUnavailable(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsTimeout(err) {
		return fallback, true
	}
	return 0, false
}
