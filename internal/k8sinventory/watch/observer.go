// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package watch // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/watch"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

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

const defaultResourceVersion = "1"

type Config struct {
	k8sinventory.Config
	IncludeInitialState    bool
	PersistResourceVersion bool
	Exclude                map[apiWatch.EventType]bool
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

	// Initialize checkpointer if storage is available and persistence is enabled
	if storageClient != nil && config.PersistResourceVersion {
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

func (o *Observer) startWatch(ctx context.Context, resource dynamic.ResourceInterface, namespace string, stopperChan chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	if o.config.IncludeInitialState {
		o.sendInitialState(ctx, resource)
	}

	watchFunc := func(options metav1.ListOptions) (apiWatch.Interface, error) {
		options.FieldSelector = o.config.FieldSelector
		options.LabelSelector = o.config.LabelSelector
		return resource.Watch(ctx, options)
	}

	cancelCtx, cancel := context.WithCancel(ctx)

	wait.UntilWithContext(cancelCtx, func(newCtx context.Context) {
		resourceVersion, err := o.getResourceVersion(newCtx, resource, namespace)
		if err != nil {
			o.logger.Error("could not retrieve a resourceVersion",
				zap.String("resource", o.config.Gvr.String()),
				zap.Error(err))
			cancel()
			return
		}

		done := o.doWatch(ctx, resourceVersion, namespace, watchFunc, stopperChan)
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
				o.logger.Warn("failed to delete persisted resourceVersion after watch restart",
					zap.String("namespace", namespace),
					zap.String("resource", o.config.Gvr.Resource),
					zap.Error(err))
			}
		}
	}, 0)
}

// sendInitialState sends the current state of objects as synthetic Added events
func (o *Observer) sendInitialState(ctx context.Context, resource dynamic.ResourceInterface) {
	o.logger.Info("sending initial state",
		zap.String("resource", o.config.Gvr.String()),
		zap.Strings("namespaces", o.config.Namespaces))

	listOption := metav1.ListOptions{
		FieldSelector: o.config.FieldSelector,
		LabelSelector: o.config.LabelSelector,
	}

	objects, err := resource.List(ctx, listOption)
	if err != nil {
		o.logger.Error("error in listing objects for initial state",
			zap.String("resource", o.config.Gvr.String()),
			zap.Error(err))
		return
	}

	if len(objects.Items) == 0 {
		o.logger.Debug("no objects found for initial state",
			zap.String("resource", o.config.Gvr.String()))
		return
	}

	// Convert each object to a synthetic Added event for consistency with watch mode
	for _, obj := range objects.Items {
		event := &apiWatch.Event{
			Type:   apiWatch.Added,
			Object: &obj,
		}

		if o.handleWatchEventFunc != nil {
			o.handleWatchEventFunc(event)
		}
	}

	o.logger.Info("initial state sent",
		zap.String("resource", o.config.Gvr.String()),
		zap.Int("object_count", len(objects.Items)))
}

// doWatch returns true when watching is done, false when watching should be restarted.
func (o *Observer) doWatch(ctx context.Context, resourceVersion, namespace string, watchFunc func(options metav1.ListOptions) (apiWatch.Interface, error), stopperChan chan struct{}) bool {
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

			// Persist resourceVersion after successfully handling the event
			if o.checkpointer != nil {
				if obj, ok := data.Object.(*unstructured.Unstructured); ok {
					rv := obj.GetResourceVersion()
					if rv != "" {
						// Use the namespace parameter which represents the watch stream scope:
						// - "" (empty) means cluster-wide watch stream - one key for all namespaces
						// - "default" means namespace-specific watch stream - one key per namespace
						// We do NOT use obj.GetNamespace() because that would create separate keys
						// for each namespace even in a cluster-wide watch, which is incorrect.
						checkpointNs := namespace

						// Save synchronously to ensure persistence before processing next event
						if err := o.checkpointer.SetCheckpoint(context.Background(), checkpointNs, o.config.Gvr.Resource, rv); err != nil {
							o.logger.Debug("failed to persist resourceVersion",
								zap.String("namespace", checkpointNs),
								zap.String("resource", o.config.Gvr.Resource),
								zap.String("resourceVersion", rv),
								zap.Error(err))
						}
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
		return "", fmt.Errorf("could not perform initial list for watch on %v, %w", o.config.Gvr.String(), err)
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
			return "", err
		}

		o.logger.Debug("retrieved resourceVersion from List() API",
			zap.String("resourceVersion", listVersion),
			zap.String("namespace", namespace),
			zap.Error(err))

		// Persist the list version for future use
		if err := o.checkpointer.SetCheckpoint(ctx, namespace, o.config.Gvr.Resource, listVersion); err != nil {
			o.logger.Warn("failed to persist initial resourceVersion",
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
