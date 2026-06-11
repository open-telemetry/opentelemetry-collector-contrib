// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package watch // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/watch"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
)

const (
	defaultResourceVersion    = "1"
	defaultCheckpointInterval = 5 * time.Second
)

type Config struct {
	k8sinventory.Config
	IncludeInitialState bool
	Exclude             map[apiWatch.EventType]bool
}

type Observer struct {
	config       Config
	checkpointer *checkpointer

	client dynamic.Interface
	logger *zap.Logger

	handleWatchEventFunc func(event *apiWatch.Event)
}

func New(client dynamic.Interface, config Config, logger *zap.Logger, storageClient storage.Client, handleWatchEventFunc func(event *apiWatch.Event)) (*Observer, error) {
	o := &Observer{
		client:               client,
		config:               config,
		logger:               logger,
		handleWatchEventFunc: handleWatchEventFunc,
	}

	// Initialize checkpointer if a storage client is provided
	if storageClient != nil {
		o.checkpointer = newCheckpointer(storageClient, logger)
	}

	return o, nil
}

func (o *Observer) Start(ctx context.Context, wg *sync.WaitGroup) chan struct{} {
	resource := o.client.Resource(o.config.Gvr)
	o.logger.Info("Started collecting",
		zap.Any("gvr", o.config.Gvr),
		zap.Any("mode", "watch"),
		zap.Any("namespaces", o.config.Namespaces))

	stopperChan := make(chan struct{})

	if len(o.config.Namespaces) == 0 {
		wg.Add(1)
		go o.startWatch(ctx, resource, "", stopperChan, wg)
	} else {
		for _, ns := range o.config.Namespaces {
			wg.Add(1)
			go o.startWatch(ctx, resource.Namespace(ns), ns, stopperChan, wg)
		}
	}

	return stopperChan
}

// startCheckpointFlusher starts a goroutine that flushes all buffered
// checkpoints to storage every defaultCheckpointInterval. It returns:
//   - setLatestRV: called by the watch loop to buffer the latest resourceVersion.
//   - flush: triggers an immediate flush, bypassing the ticker interval.
//   - stop: must be deferred by the caller; stops the ticker and does a final flush.
//
// If no checkpointer is configured, all returned functions are no-ops.
func (o *Observer) startCheckpointFlusher(ctx context.Context, namespace string) (setLatestRV func(string), flush, stop func()) {
	if o.checkpointer == nil {
		return func(string) {}, func() {}, func() {}
	}

	setLatestRV = func(rv string) {
		// Buffer the latest rv in the checkpointer; Flush will write it to storage.
		if err := o.checkpointer.SetCheckpoint(context.Background(), namespace, o.config.Gvr.Resource, rv); err != nil {
			o.logger.Error("failed to buffer resourceVersion",
				zap.String("namespace", namespace),
				zap.String("resource", o.config.Gvr.Resource),
				zap.Error(err))
		}
	}

	flush = func() {
		if err := o.checkpointer.Flush(context.Background()); err != nil {
			o.logger.Error("failed to flush checkpoints",
				zap.String("namespace", namespace),
				zap.String("resource", o.config.Gvr.Resource),
				zap.Error(err))
		}
	}

	ticker := time.NewTicker(defaultCheckpointInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				flush()
			case <-ctx.Done():
				return
			}
		}
	}()

	stop = func() {
		ticker.Stop()
		flush() // final flush to minimize replay window on restart
	}

	return setLatestRV, flush, stop
}

func (o *Observer) startWatch(ctx context.Context, resource dynamic.ResourceInterface, namespace string, stopperChan chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	watchFunc := func(options metav1.ListOptions) (apiWatch.Interface, error) {
		options.FieldSelector = o.config.FieldSelector
		options.LabelSelector = o.config.LabelSelector
		return resource.Watch(ctx, options)
	}

	cancelCtx, cancel := context.WithCancel(ctx)

	// Start flusher before sendInitialState so setLatestRV is wired up for
	// both the initial listing and the subsequent watch loop.
	setLatestRV, flushCheckpoint, stopFlusher := o.startCheckpointFlusher(cancelCtx, namespace)
	defer stopFlusher()

	// initialListRV holds the list resourceVersion returned by sendInitialState.
	// It is used as the watch starting point on the first iteration, eliminating
	// a second List() call and closing the race window between the two listings.
	// It is cleared after the first iteration so subsequent restarts (e.g. after
	// a 410 Gone) fall back to getResourceVersion() as normal.
	var initialListRV string
	if o.config.IncludeInitialState {
		initialListRV = o.sendInitialState(ctx, resource, namespace, setLatestRV)
		// Update the checkpoint with the list's own RV, which is >= any individual
		// object RV and represents the precise snapshot point of the listing.
		if initialListRV != "" {
			setLatestRV(initialListRV)
		}
		// Force-flush immediately so the rv is durable before the watch loop starts.
		flushCheckpoint()
	}

	wait.UntilWithContext(cancelCtx, func(newCtx context.Context) {
		var resourceVersion string
		if initialListRV != "" {
			// First iteration: reuse the list RV from sendInitialState directly,
			// avoiding a redundant List() call and the race window it creates.
			resourceVersion = initialListRV
			initialListRV = ""
		} else {
			var err error
			resourceVersion, err = o.getResourceVersion(newCtx, resource, namespace)
			if err != nil {
				o.logger.Error("could not retrieve a resourceVersion",
					zap.String("resource", o.config.Gvr.String()),
					zap.String("namespace", namespace),
					zap.Error(err))
				cancel()
				return
			}
		}

		done := o.doWatch(ctx, resourceVersion, namespace, watchFunc, stopperChan, setLatestRV)
		if done {
			cancel()
			return
		}

		// need to restart with a fresh resource version
		// Clear the config resourceVersion
		o.config.ResourceVersion = ""

		// Delete the persisted resourceVersion so we don't reuse the stale/expired value
		// This handles 410 Gone errors where the persisted resourceVersion is too old
		if o.checkpointer != nil {
			if err := o.checkpointer.DeleteCheckpoint(context.Background(), namespace, o.config.Gvr.Resource); err != nil {
				o.logger.Error("failed to delete persisted resourceVersion after watch restart",
					zap.String("namespace", namespace),
					zap.String("resource", o.config.Gvr.Resource),
					zap.Error(err))
			}
		}
	}, 0)
}

// sendInitialState sends the current state of objects as synthetic Added events.
// If a checkpointer is active, it retrieves the persisted resourceVersion and
// skips any object whose resourceVersion is not greater than that value —
// those objects were already seen before the last checkpoint. setLatestRV is
// called for every event that is emitted so the flusher tracks the high-water
// mark across the initial listing as well as the watch loop.
// It returns the list's own ResourceVersion, which the caller should use as
// the watch starting point to avoid a redundant List() call.
func (o *Observer) sendInitialState(ctx context.Context, resource dynamic.ResourceInterface, namespace string, setLatestRV func(string)) string {
	o.logger.Info("sending initial state",
		zap.String("resource", o.config.Gvr.String()),
		zap.Strings("namespaces", o.config.Namespaces))

	// Retrieve the persisted resourceVersion so we can skip already-seen objects.
	var persistedRV int64
	if o.checkpointer != nil {
		if rv, err := o.checkpointer.GetCheckpoint(ctx, namespace, o.config.Gvr.Resource); err == nil && rv != "" {
			o.logger.Debug("retrieved persisted resourceVersion",
				zap.String("resourceVersion", rv),
				zap.String("namespace", namespace),
				zap.String("resource", o.config.Gvr.Resource))
			if parsed, err := strconv.ParseInt(rv, 10, 64); err == nil {
				persistedRV = parsed
			}
		}
	}

	listOption := metav1.ListOptions{
		FieldSelector: o.config.FieldSelector,
		LabelSelector: o.config.LabelSelector,
	}

	objects, err := resource.List(ctx, listOption)
	if err != nil {
		o.logger.Error("error in listing objects for initial state",
			zap.String("resource", o.config.Gvr.String()),
			zap.Error(err))
		return ""
	}

	listRV := objects.GetResourceVersion()

	if len(objects.Items) == 0 {
		o.logger.Debug("no objects found for initial state",
			zap.String("resource", o.config.Gvr.String()))
		return listRV
	}

	emitted := 0
	for _, obj := range objects.Items {
		rv := obj.GetResourceVersion()

		// Skip objects that were already processed before the last checkpoint.
		// If the object's RV cannot be parsed, emit the event anyway to avoid
		// silently dropping it — a potential duplicate is safer than a missed event.
		if persistedRV > 0 {
			objRV, err := strconv.ParseInt(rv, 10, 64)
			if err != nil {
				o.logger.Warn("could not parse object resourceVersion, emitting event to avoid data loss",
					zap.String("object_resourceVersion", rv),
					zap.String("namespace", namespace),
					zap.String("resource", o.config.Gvr.String()),
					zap.Error(err))
			} else if objRV <= persistedRV {
				o.logger.Debug("skipping object already processed before last checkpoint",
					zap.String("persisted_resourceVersion", strconv.FormatInt(persistedRV, 10)),
					zap.String("object_resourceVersion", rv),
					zap.String("namespace", namespace),
					zap.String("resource", o.config.Gvr.String()))
				continue
			}
		}

		if o.handleWatchEventFunc != nil {
			o.handleWatchEventFunc(&apiWatch.Event{
				Type:   apiWatch.Added,
				Object: &obj,
			})
		}

		if rv != "" {
			setLatestRV(rv)
		}
		emitted++
	}

	o.logger.Info("initial state sent",
		zap.String("namespace", namespace),
		zap.String("list_rv", listRV),
		zap.String("resource", o.config.Gvr.String()),
		zap.Int("object_count", emitted))
	return listRV
}

// doWatch returns true when watching is done, false when watching should be restarted.
// setLatestRV is called with each new resourceVersion to update the in-memory value
// that the periodic checkpoint flush will persist.
func (o *Observer) doWatch(ctx context.Context, resourceVersion, _ string, watchFunc func(options metav1.ListOptions) (apiWatch.Interface, error), stopperChan chan struct{}, setLatestRV func(string)) bool {
	watcher, err := watch.NewRetryWatcherWithContext(ctx, resourceVersion, &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		o.logger.Error("error in watching object",
			zap.String("resource", o.config.Gvr.String()),
			zap.Error(err))
		return false
	}

	defer watcher.Stop()
	res := watcher.ResultChan()
	for {
		select {
		case data, ok := <-res:
			if data.Type == apiWatch.Error {
				errObject := apierrors.FromObject(data.Object)
				//nolint:errorlint
				if errObject.(*apierrors.StatusError).ErrStatus.Code == http.StatusGone {
					o.logger.Info("received a 410, grabbing new resource version",
						zap.Any("data", data))
					// we received a 410 so we need to restart
					return false
				}
			}

			if !ok {
				o.logger.Warn("Watch channel closed unexpectedly",
					zap.String("resource", o.config.Gvr.String()))
				return true
			}

			if o.config.Exclude[data.Type] {
				o.logger.Debug("dropping excluded data",
					zap.String("type", string(data.Type)))
				continue
			}

			if o.handleWatchEventFunc != nil {
				o.handleWatchEventFunc(&data)
			}

			// Update the in-memory resourceVersion; the periodic flush goroutine
			// will persist it to storage, avoiding a storage write per event.
			if o.checkpointer != nil {
				if obj, ok := data.Object.(*unstructured.Unstructured); ok {
					// Use the namespace parameter which represents the watch stream scope:
					// - "" (empty) means cluster-wide watch stream - one key for all namespaces
					// - "default" means namespace-specific watch stream - one key per namespace
					// We do NOT use obj.GetNamespace() because that would create separate keys
					// for each namespace even in a cluster-wide watch, which is incorrect.
					if rv := obj.GetResourceVersion(); rv != "" {
						setLatestRV(rv)
					}
				}
			}

		case <-stopperChan:
			watcher.Stop()
			return true
		}
	}
}

// fetchListResourceVersion performs a List operation and returns the latest resourceVersion.
// Returns defaultResourceVersion if the API returns an empty or zero version.
func (o *Observer) fetchListResourceVersion(ctx context.Context, resource dynamic.ResourceInterface) (string, error) {
	objects, err := resource.List(ctx, metav1.ListOptions{
		FieldSelector: o.config.FieldSelector,
		LabelSelector: o.config.LabelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("could not perform initial list for watch on %s, %w", o.config.Gvr.String(), err)
	}
	if objects == nil {
		return "", errors.New("nil objects returned, this is an error in the k8s observer")
	}

	listVersion := objects.GetResourceVersion()

	// If we still don't have a resourceVersion, use default
	if listVersion == "" || listVersion == "0" {
		listVersion = defaultResourceVersion
	}

	return listVersion, nil
}

// getResourceVersion determines the optimal resourceVersion to start watching from.
// Priority order:
// 1. Persisted resourceVersion (if checkpointer is not nil)
// 2. Config resourceVersion (if checkpointer is nil)
// 3. List resourceVersion (if no persisted/config version available)
func (o *Observer) getResourceVersion(ctx context.Context, resource dynamic.ResourceInterface, namespace string) (string, error) {
	// Priority 1: Check for persisted resourceVersion if checkpointer is available
	if o.checkpointer != nil {
		persistedVersion, err := o.checkpointer.GetCheckpoint(ctx, namespace, o.config.Gvr.Resource)
		o.logger.Debug("loading persisted resourceVersion",
			zap.String("resourceVersion", persistedVersion),
			zap.String("namespace", namespace),
			zap.Error(err))

		// If persisted version exists and is valid, use it
		if err == nil && persistedVersion != "" && persistedVersion != "0" {
			return persistedVersion, nil
		}

		// No valid persisted version found - get from List and persist it
		listVersion, err := o.fetchListResourceVersion(ctx, resource)
		if err != nil {
			o.logger.Error("failed to retrieve resourceVersion from List() API",
				zap.String("namespace", namespace),
				zap.String("resource", o.config.Gvr.String()),
				zap.Error(err))
			return "", err
		}

		o.logger.Debug("retrieved resourceVersion from List() API",
			zap.String("resourceVersion", listVersion),
			zap.String("namespace", namespace))

		// Persist the list version for future use
		if err := o.checkpointer.SetCheckpoint(ctx, namespace, o.config.Gvr.Resource, listVersion); err != nil {
			o.logger.Error("failed to persist initial resourceVersion",
				zap.String("namespace", namespace),
				zap.String("resource", o.config.Gvr.Resource),
				zap.String("resourceVersion", listVersion),
				zap.Error(err))
		}

		// Flush the checkpoint to storage immediately.
		if err := o.checkpointer.Flush(ctx); err != nil {
			o.logger.Error("failed to flush initial resourceVersion",
				zap.String("namespace", namespace),
				zap.String("resource", o.config.Gvr.Resource),
				zap.String("resourceVersion", listVersion),
				zap.Error(err))
		}

		return listVersion, nil
	}

	// Priority 2: No checkpointer - check config resourceVersion
	configVersion := o.config.ResourceVersion
	if configVersion != "" && configVersion != "0" {
		return configVersion, nil
	}
	// Priority 3: No config version - perform List operation
	listVersion, err := o.fetchListResourceVersion(ctx, resource)
	if err != nil {
		return "", err
	}
	return listVersion, nil
}
