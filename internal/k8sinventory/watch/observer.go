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
		o.config.ResourceVersion = ""
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
func (o *Observer) doWatch(ctx context.Context, resourceVersion string, namespace string, watchFunc func(options metav1.ListOptions) (apiWatch.Interface, error), stopperChan chan struct{}) bool {
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
						if err := o.checkpointer.SetResourceVersion(context.Background(), checkpointNs, o.config.Gvr.Resource, rv); err != nil {
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

// getResourceVersion determines the optimal resourceVersion to start watching from.
// If config resourceVersion is provided, it is used directly.
// Otherwise, compares List and persisted versions and uses the highest to avoid 410 Gone errors.
func (o *Observer) getResourceVersion(ctx context.Context, resource dynamic.ResourceInterface, namespace string) (string, error) {
	resourceVersion := o.config.ResourceVersion

	if resourceVersion == "" || resourceVersion == "0" {
		// Proper use of the Kubernetes API Watch capability when no resourceVersion is supplied is to do a list first
		// to get the initial state and a useable resourceVersion.
		// See https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes for details.
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

		resourceVersion = objects.GetResourceVersion()

		// If we still don't have a resourceVersion we can try 1 as a last ditch effort.
		// This also helps our unit tests since the fake client can't handle returning resource versions
		// as part of a list of objects.
		if resourceVersion == "" || resourceVersion == "0" {
			resourceVersion = defaultResourceVersion
		}

		// Load persisted version if available and compare with list version
		if o.checkpointer != nil {
			persistedVersion, err := o.checkpointer.GetResourceVersion(ctx, namespace, o.config.Gvr.Resource)
			o.logger.Debug("persisted resourceVersion ",
				zap.String("resourceVersion", persistedVersion),
				zap.String("namespace", namespace))
			if err != nil {
				o.logger.Warn("failed to load persisted resourceVersion",
					zap.String("namespace", namespace),
					zap.String("resource", o.config.Gvr.Resource),
					zap.Error(err))
			} else if persistedVersion != "" && persistedVersion != "0" {
				// Use persisted version if it's higher than list version
				if compareResourceVersions(persistedVersion, resourceVersion) > 0 {
					resourceVersion = persistedVersion
				}
			}
		}
	}

	return resourceVersion, nil
}

// compareResourceVersions compares two resourceVersion strings.
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
// Kubernetes resourceVersions are monotonically increasing integers represented as strings.
func compareResourceVersions(a, b string) int {
	// Parse as unsigned 64-bit integers for proper numeric comparison
	aInt, errA := strconv.ParseUint(a, 10, 64)
	bInt, errB := strconv.ParseUint(b, 10, 64)

	// If both parse successfully, compare as numbers
	if errA == nil && errB == nil {
		if aInt < bInt {
			return -1
		} else if aInt > bInt {
			return 1
		}
		return 0
	}

	// Fallback to string comparison if parsing fails
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}
