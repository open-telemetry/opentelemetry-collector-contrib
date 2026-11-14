// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package watch // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/watch"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
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
	"k8s.io/utils/clock"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/checkpoint"
)

const (
	defaultResourceVersion       = "1"
	defaultCheckpointInterval    = 5 * time.Second
	resourceVersionRetryInterval = 5 * time.Second
	resourceVersionRetryCap      = 60 * time.Second
	resourceVersionRetryFactor   = 2
	resourceVersionRetryJitter   = 0.2
	// watch restart backoff mirrors client-go's reflector defaults
	watchRestartInterval      = 800 * time.Millisecond
	watchRestartCap           = 30 * time.Second
	watchRestartResetInterval = 2 * time.Minute
	watchRestartFactor        = 2
	watchRestartJitter        = 1.0
	// re-list after this many restarts in a row in which the watch neither
	// moved its resourceVersion nor stayed up for watchHealthyDuration; at
	// RetryWatcher's one restart per second that is about a minute
	watchNoProgressRestarts = 60
	watchHealthyDuration    = 10 * time.Second
)

type Config struct {
	k8sinventory.Config
	IncludeInitialState bool
	Exclude             map[apiWatch.EventType]bool
}

type Observer struct {
	config       Config
	checkpointer *checkpoint.Checkpointer

	client dynamic.Interface
	logger *zap.Logger

	handleWatchEventFunc        func(event *apiWatch.Event)
	resourceVersionRetryBackoff wait.Backoff
	watchRestartBackoff         wait.Backoff
	noProgressRestarts          int
}

func New(client dynamic.Interface, config Config, logger *zap.Logger, storageClient storage.Client, handleWatchEventFunc func(event *apiWatch.Event)) (*Observer, error) {
	o := &Observer{
		client:               client,
		config:               config,
		logger:               logger,
		handleWatchEventFunc: handleWatchEventFunc,
		resourceVersionRetryBackoff: wait.Backoff{
			Duration: resourceVersionRetryInterval,
			Factor:   resourceVersionRetryFactor,
			Jitter:   resourceVersionRetryJitter,
			Cap:      resourceVersionRetryCap,
			Steps:    math.MaxInt32,
		},
		watchRestartBackoff: wait.Backoff{
			Duration: watchRestartInterval,
			Factor:   watchRestartFactor,
			Jitter:   watchRestartJitter,
			Cap:      watchRestartCap,
			Steps:    math.MaxInt32,
		},
		noProgressRestarts: watchNoProgressRestarts,
	}

	// Initialize checkpointer if a storage client is provided
	if storageClient != nil {
		o.checkpointer = checkpoint.New(storageClient, logger)
	}

	return o, nil
}

func (o *Observer) Start(ctx context.Context, wg *sync.WaitGroup) (chan struct{}, error) {
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

	return stopperChan, nil
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

	watchFunc := func(watchCtx context.Context, options metav1.ListOptions) (apiWatch.Interface, error) {
		options.FieldSelector = o.config.FieldSelector
		options.LabelSelector = o.config.LabelSelector
		return resource.Watch(watchCtx, options)
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-stopperChan:
			cancel()
		case <-ctx.Done():
		}
	}()

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

	// Copy the configured resourceVersion so a restart can drop it without
	// writing to o.config, which every namespace goroutine reads.
	configVersion := o.config.ResourceVersion
	restartDelay := o.watchRestartBackoff.DelayWithReset(clock.RealClock{}, watchRestartResetInterval)
	_ = restartDelay.Until(cancelCtx, true, true, func(newCtx context.Context) (bool, error) {
		var resourceVersion string
		if initialListRV != "" {
			// First iteration: reuse the list RV from sendInitialState directly,
			// avoiding a redundant List() call and the race window it creates.
			resourceVersion = initialListRV
			initialListRV = ""
		} else {
			var err error
			resourceVersion, err = o.getResourceVersionWithRetry(newCtx, resource, namespace, configVersion, stopperChan)
			if err != nil {
				o.logger.Error("could not retrieve a resourceVersion",
					zap.String("resource", o.config.Gvr.String()),
					zap.String("namespace", namespace),
					zap.Error(err))
				return true, nil
			}
		}

		done, expired := o.doWatch(ctx, resourceVersion, namespace, watchFunc, stopperChan, setLatestRV)
		if done {
			return true, nil
		}

		// need to restart with a fresh resource version
		configVersion = ""

		// Delete the persisted resourceVersion so we don't reuse the stale/expired value
		// This handles 410 Gone errors where the persisted resourceVersion is too old
		if expired && o.checkpointer != nil {
			if err := o.checkpointer.DeleteCheckpoint(context.Background(), namespace, o.config.Gvr.Resource); err != nil {
				o.logger.Error("failed to delete persisted resourceVersion after watch restart",
					zap.String("namespace", namespace),
					zap.String("resource", o.config.Gvr.Resource),
					zap.Error(err))
			}
		}
		return false, nil
	})
}

// getResourceVersionWithRetry retries getResourceVersion using an exponential
// backoff until it succeeds, the context is cancelled, or the stopper is signaled.
func (o *Observer) getResourceVersionWithRetry(ctx context.Context, resource dynamic.ResourceInterface, namespace, configVersion string, stopperChan chan struct{}) (string, error) {
	backoff := o.resourceVersionRetryBackoff
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-stopperChan:
			return "", context.Canceled
		default:
		}

		resourceVersion, err := o.getResourceVersion(ctx, resource, namespace, configVersion)
		if err == nil {
			return resourceVersion, nil
		}

		o.logger.Warn("could not retrieve a resourceVersion, will retry",
			zap.String("resource", o.config.Gvr.String()),
			zap.String("namespace", namespace),
			zap.Error(err))

		timer := time.NewTimer(backoff.Step())
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-stopperChan:
			timer.Stop()
			return "", context.Canceled
		case <-timer.C:
		}
	}
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

// doWatch returns whether watching is done and whether the resource version expired
// or the watch got stuck on it.
// setLatestRV is called with each new resourceVersion to update the in-memory value
// that the periodic checkpoint flush will persist.
func (o *Observer) doWatch(ctx context.Context, resourceVersion, namespace string, watchFunc func(ctx context.Context, options metav1.ListOptions) (apiWatch.Interface, error), stopperChan chan struct{}, setLatestRV func(string)) (done, expired bool) {
	stall := newStallDetector(o.noProgressRestarts)
	watcher, err := watch.NewRetryWatcherWithContext(ctx, resourceVersion, &cache.ListWatch{WatchFuncWithContext: stall.wrap(watchFunc)})
	if err != nil {
		o.logger.Error("error in watching object",
			zap.String("resource", o.config.Gvr.String()),
			zap.Error(err))
		return false, false
	}

	defer watcher.Stop()
	res := watcher.ResultChan()
	for {
		select {
		case data, ok := <-res:
			if !ok {
				o.logger.Warn("Watch channel closed unexpectedly",
					zap.String("resource", o.config.Gvr.String()))
				return false, false
			}

			if data.Type == apiWatch.Error {
				errObject := apierrors.FromObject(data.Object)
				if apierrors.IsResourceExpired(errObject) || apierrors.IsGone(errObject) {
					o.logger.Info("received a 410, grabbing new resource version",
						zap.Any("data", data))
					// we received a 410 so we need to restart
					return false, true
				}
				o.logger.Warn("received watch error event",
					zap.String("resource", o.config.Gvr.String()),
					zap.String("reason", string(apierrors.ReasonForError(errObject))),
					zap.Error(errObject))
				continue
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

		case <-stall.detected:
			fields := []zap.Field{
				zap.String("resource", o.config.Gvr.String()),
				zap.String("namespace", namespace),
				zap.Error(stall.err),
			}
			if apierrors.IsResourceExpired(stall.err) || apierrors.IsGone(stall.err) {
				o.logger.Info("received a 410, grabbing new resource version", fields...)
			} else {
				o.logger.Warn("watch is stuck, grabbing new resource version", fields...)
			}
			return false, true

		case <-stopperChan:
			watcher.Stop()
			return true, false
		}
	}
}

// stallDetector wraps the watch function used by client-go's RetryWatcher.
// RetryWatcher keeps retrying from the same resourceVersion when the watch
// request fails with 410 Gone, or when every attempt fails or ends before
// delivering anything, and it never closes its result channel in either case.
// stallDetector closes detected instead so that doWatch can re-list.
type stallDetector struct {
	maxRestarts int
	now         func() time.Time

	detected chan struct{}
	err      error // set before detected is closed

	// RetryWatcher runs one attempt at a time on a single goroutine, so these
	// need no locking.
	restarts  int
	lastRV    string
	lastStart time.Time

	// unavailable is set when an attempt fails because the apiserver cannot
	// serve the watch right now (see isAPIServerUnavailable). A re-list would
	// not help there, so such an attempt does not count as a restart without
	// progress.
	unavailable atomic.Bool
}

func newStallDetector(maxRestarts int) *stallDetector {
	return &stallDetector{
		maxRestarts: maxRestarts,
		now:         time.Now,
		detected:    make(chan struct{}),
	}
}

func (d *stallDetector) stall(err error) {
	d.err = err
	close(d.detected)
}

func (d *stallDetector) wrap(watchFunc func(context.Context, metav1.ListOptions) (apiWatch.Interface, error)) func(context.Context, metav1.ListOptions) (apiWatch.Interface, error) {
	return func(ctx context.Context, options metav1.ListOptions) (apiWatch.Interface, error) {
		if d.err != nil {
			return nil, d.err
		}

		// RetryWatcher calls this once per attempt and only moves the
		// resourceVersion on events and newer bookmarks, so the previous
		// attempt made progress if the version moved or it stayed up long
		// enough.
		now := d.now()
		unavailable := d.unavailable.Swap(false)
		switch {
		case options.ResourceVersion != d.lastRV || now.Sub(d.lastStart) >= watchHealthyDuration:
			d.restarts = 0
		case !unavailable:
			d.restarts++
		}
		d.lastRV, d.lastStart = options.ResourceVersion, now
		if d.restarts >= d.maxRestarts {
			d.stall(fmt.Errorf("watch restarted %d times without progress from resourceVersion %s", d.restarts, options.ResourceVersion))
			return nil, d.err
		}

		w, err := watchFunc(ctx, options)
		switch {
		case apierrors.IsResourceExpired(err) || apierrors.IsGone(err):
			d.stall(err)
		case isAPIServerUnavailable(err):
			d.unavailable.Store(true)
		case err == nil && w != nil:
			return d.forward(w), nil
		}
		return w, err
	}
}

// forward relays the events of w, noting in-band errors that mean the
// apiserver is unavailable. RetryWatcher retries those itself, so they never
// reach doWatch.
func (d *stallDetector) forward(w apiWatch.Interface) apiWatch.Interface {
	ch := make(chan apiWatch.Event)
	proxy := apiWatch.NewProxyWatcher(ch)
	go func() {
		defer close(ch)
		defer w.Stop()
		for {
			select {
			case ev, ok := <-w.ResultChan():
				if !ok {
					return
				}
				if ev.Type == apiWatch.Error && isAPIServerUnavailable(apierrors.FromObject(ev.Object)) {
					d.unavailable.Store(true)
				}
				select {
				case ch <- ev:
				case <-proxy.StopChan():
					return
				}
			case <-proxy.StopChan():
				return
			}
		}
	}()
	return proxy
}

// isAPIServerUnavailable reports whether err means the apiserver could not be
// reached or would not serve the watch right now: a network error, or a 401,
// 403, 429, 503, 504 or server timeout response. A re-list would fail the same
// way, so the watch keeps its resourceVersion. A 504 for a resourceVersion
// newer than the server has is left out, since a re-list does fix that.
func isAPIServerUnavailable(err error) bool {
	var netErr net.Error
	switch {
	case errors.As(err, &netErr):
		return true
	case apierrors.HasStatusCause(err, metav1.CauseTypeResourceVersionTooLarge):
		return false
	}
	return apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) || apierrors.IsTooManyRequests(err) ||
		apierrors.IsServiceUnavailable(err) || apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err)
}

// fetchListResourceVersion performs a List operation and returns the latest resourceVersion.
// Returns defaultResourceVersion if the API returns an empty or zero version.
func (o *Observer) fetchListResourceVersion(ctx context.Context, resource dynamic.ResourceInterface) (string, error) {
	objects, err := resource.List(ctx, metav1.ListOptions{
		FieldSelector: o.config.FieldSelector,
		LabelSelector: o.config.LabelSelector,
		Limit:         1,
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
func (o *Observer) getResourceVersion(ctx context.Context, resource dynamic.ResourceInterface, namespace, configVersion string) (string, error) {
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
