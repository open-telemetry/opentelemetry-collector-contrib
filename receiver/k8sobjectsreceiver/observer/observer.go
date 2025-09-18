package observer

import (
	"context"
	"errors"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
	"net/http"
	"sync"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"time"
)

type Mode string

const (
	PullMode  Mode = "pull"
	WatchMode Mode = "watch"

	DefaultMode = PullMode

	defaultResourceVersion = "1"
)

type Config struct {
	ApiConfig           k8sconfig.APIConfig
	Gvr                 schema.GroupVersionResource
	Mode                Mode
	Namespaces          []string
	Interval            time.Duration
	LabelSelector       string
	FieldSelector       string
	ResourceVersion     string
	IncludeInitialState bool
	Exclude             map[apiWatch.EventType]bool

	HandleWatchEventFunc  func(event *apiWatch.Event)
	HandlePullObjectsFunc func(objects *unstructured.UnstructuredList)
}

type Observer struct {
	config Config

	client          dynamic.Interface
	logger          *zap.Logger
	stopperChanList []chan struct{}
	mu              sync.Mutex
}

func New(client dynamic.Interface, config Config, logger *zap.Logger) (*Observer, error) {
	o := &Observer{
		client:          client,
		config:          config,
		logger:          logger,
		stopperChanList: []chan struct{}{},
	}
	return o, nil
}

func (o *Observer) Start(ctx context.Context) {
	resource := o.client.Resource(o.config.Gvr)
	o.logger.Info("Started collecting",
		zap.Any("gvr", o.config.Gvr),
		zap.Any("mode", o.config.Mode),
		zap.Any("namespaces", o.config.Namespaces))

	switch o.config.Mode {
	case PullMode:
		if len(o.config.Namespaces) == 0 {
			go o.startPull(ctx, resource)
		} else {
			for _, ns := range o.config.Namespaces {
				go o.startPull(ctx, resource.Namespace(ns))
			}
		}

	case WatchMode:
		if len(o.config.Namespaces) == 0 {
			go o.startWatch(ctx, resource)
		} else {
			for _, ns := range o.config.Namespaces {
				go o.startWatch(ctx, resource.Namespace(ns))
			}
		}
	}
}

func (o *Observer) startPull(ctx context.Context, resource dynamic.ResourceInterface) {
	stopperChan := make(chan struct{})
	o.mu.Lock()
	o.stopperChanList = append(o.stopperChanList, stopperChan)
	o.mu.Unlock()
	ticker := newTicker(ctx, o.config.Interval)
	listOption := metav1.ListOptions{
		FieldSelector: o.config.FieldSelector,
		LabelSelector: o.config.LabelSelector,
	}

	if o.config.ResourceVersion != "" {
		listOption.ResourceVersion = o.config.ResourceVersion
		listOption.ResourceVersionMatch = metav1.ResourceVersionMatchExact
	}

	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			objects, err := resource.List(ctx, listOption)
			if err != nil {
				o.logger.Error("error in pulling object",
					zap.String("resource", o.config.Gvr.String()),
					zap.Error(err))
			} else if len(objects.Items) > 0 {
				if o.config.HandlePullObjectsFunc != nil {
					o.config.HandlePullObjectsFunc(objects)
				}
			}
		case <-stopperChan:
			return
		}
	}
}

func (o *Observer) startWatch(ctx context.Context, resource dynamic.ResourceInterface) {
	stopperChan := make(chan struct{})
	o.mu.Lock()
	o.stopperChanList = append(o.stopperChanList, stopperChan)
	o.mu.Unlock()

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
		resourceVersion, err := getResourceVersion(newCtx, o.config, resource)
		if err != nil {
			o.logger.Error("could not retrieve a resourceVersion",
				zap.String("resource", o.config.Gvr.String()),
				zap.Error(err))
			cancel()
			return
		}

		done := o.doWatch(newCtx, resourceVersion, watchFunc, stopperChan)
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

		if o.config.HandleWatchEventFunc != nil {
			o.config.HandleWatchEventFunc(event)
		}
	}

	o.logger.Info("initial state sent",
		zap.String("resource", o.config.Gvr.String()),
		zap.Int("object_count", len(objects.Items)))
}

// doWatch returns true when watching is done, false when watching should be restarted.
func (o *Observer) doWatch(ctx context.Context, resourceVersion string, watchFunc func(options metav1.ListOptions) (apiWatch.Interface, error), stopperChan chan struct{}) bool {
	watcher, err := watch.NewRetryWatcher(resourceVersion, &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		o.logger.Error("error in watching object",
			zap.String("resource", o.config.Gvr.String()),
			zap.Error(err))
		return true
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

			if o.config.HandleWatchEventFunc != nil {
				o.config.HandleWatchEventFunc(&data)
			}

		case <-stopperChan:
			watcher.Stop()
			return true
		}
	}
}

// Start ticking immediately.
// Ref: https://stackoverflow.com/questions/32705582/how-to-get-time-tick-to-tick-immediately
func newTicker(ctx context.Context, repeat time.Duration) *time.Ticker {
	ticker := time.NewTicker(repeat)
	oc := ticker.C
	nc := make(chan time.Time, 1)
	go func() {
		nc <- time.Now()
		for {
			select {
			case tm := <-oc:
				nc <- tm
			case <-ctx.Done():
				return
			}
		}
	}()

	ticker.C = nc
	return ticker
}

func getResourceVersion(ctx context.Context, config Config, resource dynamic.ResourceInterface) (string, error) {
	resourceVersion := config.ResourceVersion
	if resourceVersion == "" || resourceVersion == "0" {
		// Proper use of the Kubernetes API Watch capability when no resourceVersion is supplied is to do a list first
		// to get the initial state and a useable resourceVersion.
		// See https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes for details.
		objects, err := resource.List(ctx, metav1.ListOptions{
			FieldSelector: config.FieldSelector,
			LabelSelector: config.LabelSelector,
		})
		if err != nil {
			return "", fmt.Errorf("could not perform initial list for watch on %v, %w", config.Gvr.String(), err)
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
	}
	return resourceVersion, nil
}
