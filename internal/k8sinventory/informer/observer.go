// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package informer // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/informer"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
)

type Config struct {
	k8sinventory.Config
	Mode                k8sinventory.Mode
	Interval            time.Duration               // pull mode only
	IncludeInitialState bool                        // watch mode only
	Exclude             map[apiWatch.EventType]bool // watch mode only
}

// Observer uses a SharedInformerFactory; one factory per namespace.
type Observer struct {
	factories    []dynamicinformer.DynamicSharedInformerFactory
	infs         []cache.SharedIndexInformer
	config       Config
	pullHandler  func(*unstructured.UnstructuredList)
	watchHandler func(*apiWatch.Event)
	logger       *zap.Logger
}

// New creates an Observer. Watch-mode event handlers are registered here, before
// factory.Start(), to prevent the informer from replaying the cache to late-registered handlers.
func New(
	factories []dynamicinformer.DynamicSharedInformerFactory,
	config Config,
	logger *zap.Logger,
	pullHandler func(*unstructured.UnstructuredList),
	watchHandler func(*apiWatch.Event),
) (*Observer, error) {
	if len(factories) == 0 {
		return nil, errors.New("at least one SharedInformerFactory is required")
	}
	if config.Exclude[apiWatch.Bookmark] {
		logger.Warn("exclude_watch_type: BOOKMARK has no effect with informer-based watching; informers never surface BOOKMARK events")
	}

	o := &Observer{
		factories:    factories,
		infs:         make([]cache.SharedIndexInformer, len(factories)),
		config:       config,
		pullHandler:  pullHandler,
		watchHandler: watchHandler,
		logger:       logger,
	}

	for i, factory := range factories {
		inf := factory.ForResource(config.Gvr)
		o.infs[i] = inf.Informer()

		if config.Mode == k8sinventory.WatchMode {
			// include_initial_state=false: suppress isInInitialList=true adds so only post-sync events appear.
			if _, err := inf.Informer().AddEventHandler(o.buildEventHandler(!config.IncludeInitialState)); err != nil {
				return nil, fmt.Errorf("failed to add event handler for %s: %w", config.Gvr.String(), err)
			}
		}
	}

	return o, nil
}

// Start starts all informers and blocks until their caches are warm.
// ctx is shared across all observers in the receiver, so all factories stop together.
func (o *Observer) Start(ctx context.Context, wg *sync.WaitGroup) chan struct{} {
	o.logger.Info("starting informer and waiting for cache sync",
		zap.String("gvr", o.config.Gvr.String()),
		zap.String("mode", string(o.config.Mode)))

	for _, factory := range o.factories {
		factory.Start(ctx.Done())
	}

	for _, factory := range o.factories {
		syncResult := factory.WaitForCacheSync(ctx.Done())
		for gvr, synced := range syncResult {
			if !synced {
				o.logger.Error("informer cache failed to sync", zap.String("gvr", gvr.String()))
			}
		}
	}

	o.logger.Info("informer cache synced, observer ready",
		zap.String("gvr", o.config.Gvr.String()),
		zap.String("mode", string(o.config.Mode)))

	stopCh := make(chan struct{})
	if o.config.Mode == k8sinventory.PullMode {
		for _, inf := range o.infs {
			wg.Add(1)
			go o.runPullTicker(ctx, inf.GetStore(), stopCh, wg)
		}
	}

	return stopCh
}

func (o *Observer) buildEventHandler(skipInitialList bool) cache.ResourceEventHandler {
	if skipInitialList {
		return cache.ResourceEventHandlerDetailedFuncs{
			AddFunc: func(obj any, isInInitialList bool) {
				if isInInitialList {
					return
				}
				o.handleWatchEvent(apiWatch.Added, obj)
			},
			UpdateFunc: func(_, newObj any) {
				o.handleWatchEvent(apiWatch.Modified, newObj)
			},
			DeleteFunc: func(obj any) {
				o.handleWatchEvent(apiWatch.Deleted, obj)
			},
		}
	}
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			o.handleWatchEvent(apiWatch.Added, obj)
		},
		UpdateFunc: func(_, newObj any) {
			o.handleWatchEvent(apiWatch.Modified, newObj)
		},
		DeleteFunc: func(obj any) {
			o.handleWatchEvent(apiWatch.Deleted, obj)
		},
	}
}

func (o *Observer) handleWatchEvent(eventType apiWatch.EventType, obj any) {
	if o.config.Exclude[eventType] {
		return
	}
	// Unwrap tombstone objects produced when a delete is missed and detected on re-list.
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		o.logger.Error("unexpected object type in watch event", zap.String("type", string(eventType)))
		return
	}
	if o.watchHandler != nil {
		o.watchHandler(&apiWatch.Event{Type: eventType, Object: u})
	}
}

func (o *Observer) runPullTicker(ctx context.Context, store cache.Store, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	o.emitSnapshot(store)

	ticker := time.NewTicker(o.config.Interval)
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
		return // skip empty snapshots
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
	if o.pullHandler != nil {
		o.pullHandler(list)
	}
}
