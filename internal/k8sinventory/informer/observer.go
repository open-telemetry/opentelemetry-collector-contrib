// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package informer // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/informer"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/checkpoint"
)

const checkpointFlushInterval = 5 * time.Second

// PullConfig configures an informer-based pull observer.
type PullConfig struct {
	k8sinventory.Config
	Interval         time.Duration
	CacheSyncTimeout time.Duration
}

// WatchConfig configures an informer-based watch observer.
type WatchConfig struct {
	k8sinventory.Config
	IncludeInitialState bool
	Exclude             map[apiWatch.EventType]bool
	CacheSyncTimeout    time.Duration
	StorageClient       storage.Client
}

// namespacedFactory pairs a factory with its namespace (empty = cluster-wide).
type namespacedFactory struct {
	namespace string
	factory   dynamicinformer.DynamicSharedInformerFactory
}

// Observer uses SharedInformerFactories to watch or pull Kubernetes objects.
type Observer struct {
	infs             map[namespacedFactory]cache.SharedIndexInformer
	handlerRegs      map[namespacedFactory]cache.ResourceEventHandlerRegistration
	base             k8sinventory.Config
	cacheSyncTimeout time.Duration
	// Shared stop channel from the registry; closing it stops every observer at once.
	stopCh <-chan struct{}
	// pull-only
	pullHandler func(*unstructured.UnstructuredList)
	interval    time.Duration
	// watch-only
	watchHandler    func(*apiWatch.Event)
	exclude         map[apiWatch.EventType]bool
	skipInitialList bool
	cp              *checkpoint.Checkpointer
	logger          *zap.Logger
}

// NewPull creates a pull-mode observer.
func NewPull(reg *FactoryRegistry, config PullConfig, logger *zap.Logger, handler func(*unstructured.UnstructuredList)) (*Observer, error) {
	factories, err := factoriesForConfig(reg, config.Config)
	if err != nil {
		return nil, err
	}
	if len(factories) == 0 {
		return nil, errors.New("at least one factory is required")
	}
	o := &Observer{
		infs:             make(map[namespacedFactory]cache.SharedIndexInformer, len(factories)),
		base:             config.Config,
		cacheSyncTimeout: config.CacheSyncTimeout,
		stopCh:           reg.StopCh(),
		pullHandler:      handler,
		interval:         config.Interval,
		logger:           logger,
	}
	for _, nf := range factories {
		o.infs[nf] = nf.factory.ForResource(config.Gvr).Informer()
	}
	return o, nil
}

// NewWatch creates a watch-mode observer.
func NewWatch(reg *FactoryRegistry, config WatchConfig, logger *zap.Logger, handler func(*apiWatch.Event)) (*Observer, error) {
	factories, err := factoriesForConfig(reg, config.Config)
	if err != nil {
		return nil, err
	}
	if len(factories) == 0 {
		return nil, errors.New("at least one factory is required")
	}
	if config.Exclude[apiWatch.Bookmark] {
		logger.Warn("exclude_watch_type: BOOKMARK has no effect with informer-based watching; informers never surface BOOKMARK events")
	}
	o := &Observer{
		infs:             make(map[namespacedFactory]cache.SharedIndexInformer, len(factories)),
		handlerRegs:      make(map[namespacedFactory]cache.ResourceEventHandlerRegistration, len(factories)),
		base:             config.Config,
		cacheSyncTimeout: config.CacheSyncTimeout,
		stopCh:           reg.StopCh(),
		watchHandler:     handler,
		exclude:          config.Exclude,
		skipInitialList:  !config.IncludeInitialState,
		logger:           logger,
	}
	if config.StorageClient != nil {
		o.cp = checkpoint.New(config.StorageClient, logger)
	}
	for _, nf := range factories {
		o.infs[nf] = nf.factory.ForResource(config.Gvr).Informer()
	}
	return o, nil
}

// factoriesForConfig resolves one factory per configured namespace from the registry.
func factoriesForConfig(reg *FactoryRegistry, config k8sinventory.Config) ([]namespacedFactory, error) {
	namespaces := config.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""}
	}
	factories := make([]namespacedFactory, 0, len(namespaces))
	seen := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		if _, ok := seen[ns]; ok {
			continue
		}
		seen[ns] = struct{}{}
		f, err := reg.Get(ns, config.LabelSelector, config.FieldSelector)
		if err != nil {
			return nil, err
		}
		factories = append(factories, namespacedFactory{namespace: ns, factory: f})
	}
	return factories, nil
}

// Start starts the observer's factories, waits for cache sync, then launches workers.
// Safe to call across observers that share a factory: factory.Start is idempotent.
func (o *Observer) Start(ctx context.Context, wg *sync.WaitGroup) (chan struct{}, error) {
	o.logger.Info("starting informer and waiting for cache sync",
		zap.String("gvr", o.base.Gvr.String()))

	if o.cp != nil {
		namespaces := make([]string, 0, len(o.infs))
		for nf := range o.infs {
			namespaces = append(namespaces, nf.namespace)
		}
		if err := o.cp.Load(ctx, namespaces, o.base.Gvr.Resource); err != nil {
			o.logger.Warn("failed to load checkpoints from storage",
				zap.String("gvr", o.base.Gvr.String()), zap.Error(err))
		}
	}

	if o.watchHandler != nil {
		for nf, inf := range o.infs {
			handle, err := inf.AddEventHandler(o.buildEventHandler(o.skipInitialList, nf.namespace))
			if err != nil {
				o.removeHandlers()
				return nil, fmt.Errorf("failed to add event handler for %s: %w", o.base.Gvr.String(), err)
			}
			o.handlerRegs[nf] = handle
		}
	}

	for nf := range o.infs {
		nf.factory.Start(o.stopCh)
	}

	if err := o.waitForCacheSync(ctx); err != nil {
		o.removeHandlers()
		return nil, err
	}

	o.logger.Info("informer cache synced, observer ready",
		zap.String("gvr", o.base.Gvr.String()))

	if o.cp != nil {
		for nf, inf := range o.infs {
			if listRV := inf.LastSyncResourceVersion(); listRV != "" {
				if err := o.cp.SetCheckpoint(ctx, nf.namespace, o.base.Gvr.Resource, listRV); err != nil {
					o.logger.Warn("failed to persist post-sync checkpoint",
						zap.String("namespace", nf.namespace),
						zap.String("gvr", o.base.Gvr.String()),
						zap.Error(err))
					continue
				}
				if err := o.cp.Flush(ctx); err != nil {
					o.logger.Warn("failed to flush post-sync checkpoint",
						zap.String("namespace", nf.namespace),
						zap.String("gvr", o.base.Gvr.String()),
						zap.Error(err))
				}
			}
		}
	}

	stopCh := make(chan struct{})

	// Informers keep running under the registry stopCh; unsubscribe our handlers when the observer stops.
	if o.watchHandler != nil {
		wg.Add(1)
		go o.runHandlerRemover(ctx, stopCh, wg)
	}

	if o.cp != nil {
		wg.Add(1)
		go o.runCheckpointFlusher(ctx, stopCh, wg)
	}

	if o.pullHandler != nil {
		for _, inf := range o.infs {
			wg.Add(1)
			go o.runPullTicker(ctx, inf.GetStore(), stopCh, wg)
		}
	}

	return stopCh, nil
}

// waitForCacheSync waits for every informer (or handler, for watch observers) to sync.
func (o *Observer) waitForCacheSync(ctx context.Context) error {
	syncCtx, syncCancel := context.WithTimeout(ctx, o.cacheSyncTimeout)
	defer syncCancel()

	type namedSync struct {
		namespace string
		hasSynced func() bool
	}
	syncs := make([]namedSync, 0, len(o.infs))
	if o.watchHandler != nil {
		for nf, handle := range o.handlerRegs {
			syncs = append(syncs, namedSync{namespace: nf.namespace, hasSynced: handle.HasSynced})
		}
	} else {
		for nf, inf := range o.infs {
			syncs = append(syncs, namedSync{namespace: nf.namespace, hasSynced: inf.HasSynced})
		}
	}

	var errs []error
	for _, s := range syncs {
		if !cache.WaitForCacheSync(syncCtx.Done(), s.hasSynced) {
			if ctx.Err() != nil {
				errs = append(errs, fmt.Errorf("informer cache sync aborted for %s (namespace=%q): %w", o.base.Gvr, s.namespace, ctx.Err()))
			} else {
				errs = append(errs, fmt.Errorf("timed out waiting for informer cache to sync for %s (namespace=%q)", o.base.Gvr, s.namespace))
			}
		}
	}
	return errors.Join(errs...)
}

// runHandlerRemover unsubscribes this observer's ResourceEventHandlers on any
// stop signal. An in-flight callback may complete after removal.
func (o *Observer) runHandlerRemover(ctx context.Context, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	select {
	case <-stopCh:
	case <-ctx.Done():
	case <-o.stopCh:
	}
	o.removeHandlers()
}

func (o *Observer) removeHandlers() {
	for nf, handle := range o.handlerRegs {
		if err := o.infs[nf].RemoveEventHandler(handle); err != nil {
			o.logger.Warn("failed to remove event handler",
				zap.String("namespace", nf.namespace),
				zap.String("gvr", o.base.Gvr.String()),
				zap.Error(err))
		}
	}
}

func (o *Observer) buildEventHandler(skipInitialList bool, namespace string) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerDetailedFuncs{
		AddFunc: func(obj any, isInInitialList bool) {
			if isInInitialList && o.suppressInitialListEvent(obj, skipInitialList, namespace) {
				return
			}
			o.handleWatchEvent(apiWatch.Added, obj, namespace)
		},
		UpdateFunc: func(_, newObj any) {
			o.handleWatchEvent(apiWatch.Modified, newObj, namespace)
		},
		DeleteFunc: func(obj any) {
			o.handleWatchEvent(apiWatch.Deleted, obj, namespace)
		},
	}
}

func (o *Observer) suppressInitialListEvent(obj any, skipInitialList bool, namespace string) bool {
	if skipInitialList {
		return true
	}
	if o.cp == nil {
		return false
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return false
	}
	seen, err := o.cp.AlreadySeen(u.GetResourceVersion(), namespace)
	if err != nil {
		o.logger.Warn("failed to check if object was already seen", zap.Error(err))
		return false
	}
	return seen
}

func (o *Observer) handleWatchEvent(eventType apiWatch.EventType, obj any, namespace string) {
	if o.exclude[eventType] {
		return
	}
	// Unwrap tombstone objects produced when a delete is missed and detected on re-list.
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		o.logger.Error("unexpected object type in watch event",
			zap.String("event_type", string(eventType)),
			zap.String("go_type", fmt.Sprintf("%T", obj)),
			zap.String("gvr", o.base.Gvr.String()))
		return
	}
	// Deliver first, then checkpoint: a crash between the two replays rather than skips.
	o.watchHandler(&apiWatch.Event{Type: eventType, Object: u})
	if o.cp != nil {
		if rv := u.GetResourceVersion(); rv != "" {
			if err := o.cp.SetCheckpoint(context.Background(), namespace, o.base.Gvr.Resource, rv); err != nil {
				o.logger.Warn("failed to buffer resourceVersion checkpoint",
					zap.String("namespace", namespace),
					zap.String("gvr", o.base.Gvr.String()),
					zap.Error(err))
			}
		}
	}
}

func (o *Observer) runCheckpointFlusher(ctx context.Context, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(checkpointFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := o.cp.Flush(ctx); err != nil {
				o.logger.Error("failed to flush checkpoints", zap.Error(err))
			}
		case <-stopCh:
			if err := o.cp.Flush(context.Background()); err != nil {
				o.logger.Error("failed to flush final checkpoints", zap.Error(err))
			}
			return
		case <-ctx.Done():
			if err := o.cp.Flush(context.Background()); err != nil {
				o.logger.Error("failed to flush final checkpoints", zap.Error(err))
			}
			return
		}
	}
}

func (o *Observer) runPullTicker(ctx context.Context, store cache.Store, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	o.emitSnapshot(store)
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			o.emitSnapshot(store)
		case <-stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (o *Observer) emitSnapshot(store cache.Store) {
	items := store.List()
	if len(items) == 0 {
		return
	}
	list := &unstructured.UnstructuredList{}
	for _, item := range items {
		u, ok := item.(*unstructured.Unstructured)
		if !ok {
			o.logger.Error("unexpected item type in informer cache")
			continue
		}
		list.Items = append(list.Items, *u)
	}
	o.pullHandler(list)
}
