// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/internal/metadata"
)

type k8sobjectsreceiver struct {
	setting         receiver.Settings
	config          *Config
	objects         []*K8sObjectsConfig
	stopperChanList []chan struct{}
	client          dynamic.Interface
	consumer        consumer.Logs
	obsrecv         *receiverhelper.ObsReport
	mu              sync.Mutex
	cancel          context.CancelFunc
}

func newReceiver(params receiver.Settings, config *Config, consumer consumer.Logs) (receiver.Logs, error) {
	transport := "http"

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             params.ID,
		Transport:              transport,
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}

	objects := make([]*K8sObjectsConfig, len(config.Objects))
	for i, obj := range config.Objects {
		objects[i] = obj.DeepCopy()
		objects[i].exclude = make(map[apiWatch.EventType]bool)
		for _, item := range objects[i].ExcludeWatchType {
			objects[i].exclude[item] = true
		}
		// Set default interval if in PullMode and interval is 0
		if objects[i].Mode == PullMode && objects[i].Interval == 0 {
			objects[i].Interval = defaultPullInterval
		}
	}

	return &k8sobjectsreceiver{
		setting:  params,
		config:   config,
		objects:  objects,
		consumer: consumer,
		obsrecv:  obsrecv,
		mu:       sync.Mutex{},
	}, nil
}

func (kr *k8sobjectsreceiver) Start(ctx context.Context, host component.Host) error {
	client, err := kr.config.getDynamicClient()
	if err != nil {
		return err
	}
	kr.client = client

	// Validate objects against K8s API
	validObjects, err := kr.config.getValidObjects()
	if err != nil {
		return err
	}

	var validConfigs []*K8sObjectsConfig
	for _, object := range kr.objects {
		gvrs, ok := validObjects[object.Name]
		if !ok {
			availableResource := make([]string, 0, len(validObjects))
			for k := range validObjects {
				availableResource = append(availableResource, k)
			}
			err = fmt.Errorf("resource not found: %s. Available resources in cluster: %v", object.Name, availableResource)
			if handlerErr := kr.handleError(err, ""); handlerErr != nil {
				return handlerErr
			}
			continue
		}

		gvr := gvrs[0]
		for i := range gvrs {
			if gvrs[i].Group == object.Group {
				gvr = gvrs[i]
				break
			}
		}

		object.gvr = gvr
		validConfigs = append(validConfigs, object)
	}

	if len(validConfigs) == 0 {
		err = errors.New("no valid Kubernetes objects found to watch")
		return err
	}

	if kr.config.K8sLeaderElector != nil {
		k8sLeaderElector := host.GetExtensions()[*kr.config.K8sLeaderElector]
		if k8sLeaderElector == nil {
			return fmt.Errorf("unknown k8s leader elector %q", kr.config.K8sLeaderElector)
		}

		kr.setting.Logger.Info("registering the receiver in leader election")
		elector, ok := k8sLeaderElector.(k8sleaderelector.LeaderElection)
		if !ok {
			return fmt.Errorf("the extension %T is not implement k8sleaderelector.LeaderElection", k8sLeaderElector)
		}

		elector.SetCallBackFuncs(
			func(ctx context.Context) {
				cctx, cancel := context.WithCancel(ctx)
				kr.cancel = cancel
				for _, object := range validConfigs {
					kr.start(cctx, object)
				}
				kr.setting.Logger.Info("Object Receiver started as leader")
			},
			func() {
				kr.setting.Logger.Info("no longer leader, stopping")
				err = kr.Shutdown(context.Background())
				if err != nil {
					kr.setting.Logger.Error("shutdown receiver error:", zap.Error(err))
				}
			})
	} else {
		cctx, cancel := context.WithCancel(ctx)
		kr.cancel = cancel
		for _, object := range validConfigs {
			kr.start(cctx, object)
		}
	}

	return nil
}

func (kr *k8sobjectsreceiver) Shutdown(context.Context) error {
	kr.setting.Logger.Info("Object Receiver stopped")
	if kr.cancel != nil {
		kr.cancel()
	}

	kr.mu.Lock()
	for _, stopperChan := range kr.stopperChanList {
		close(stopperChan)
	}
	kr.mu.Unlock()
	return nil
}

func (kr *k8sobjectsreceiver) start(ctx context.Context, object *K8sObjectsConfig) {
	resource := kr.client.Resource(*object.gvr)
	kr.setting.Logger.Info("Started collecting",
		zap.Any("gvr", object.gvr),
		zap.Any("mode", object.Mode),
		zap.Any("namespaces", object.Namespaces))

	switch object.Mode {
	case PullMode:
		if len(object.Namespaces) == 0 {
			go kr.startPull(ctx, object, resource)
		} else {
			for _, ns := range object.Namespaces {
				go kr.startPull(ctx, object, resource.Namespace(ns))
			}
		}

	case WatchMode:
		if len(object.Namespaces) == 0 {
			go kr.startWatch(ctx, object, resource)
		} else {
			for _, ns := range object.Namespaces {
				go kr.startWatch(ctx, object, resource.Namespace(ns))
			}
		}
	}
}

func (kr *k8sobjectsreceiver) startPull(ctx context.Context, config *K8sObjectsConfig, resource dynamic.ResourceInterface) {
	stopperChan := make(chan struct{})
	kr.mu.Lock()
	kr.stopperChanList = append(kr.stopperChanList, stopperChan)
	kr.mu.Unlock()
	ticker := newTicker(ctx, config.Interval)
	listOption := metav1.ListOptions{
		FieldSelector: config.FieldSelector,
		LabelSelector: config.LabelSelector,
	}

	if config.ResourceVersion != "" {
		listOption.ResourceVersion = config.ResourceVersion
		listOption.ResourceVersionMatch = metav1.ResourceVersionMatchExact
	}

	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			objects, err := resource.List(ctx, listOption)
			if err != nil {
				kr.setting.Logger.Error("error in pulling object",
					zap.String("resource", config.gvr.String()),
					zap.Error(err))
			} else if len(objects.Items) > 0 {
				logs := pullObjectsToLogData(objects, time.Now(), config)
				obsCtx := kr.obsrecv.StartLogsOp(ctx)
				logRecordCount := logs.LogRecordCount()
				err = kr.consumer.ConsumeLogs(obsCtx, logs)
				kr.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), logRecordCount, err)
			}
		case <-stopperChan:
			return
		}
	}
}

func (kr *k8sobjectsreceiver) startWatch(ctx context.Context, config *K8sObjectsConfig, resource dynamic.ResourceInterface) {
	stopperChan := make(chan struct{})
	kr.mu.Lock()
	kr.stopperChanList = append(kr.stopperChanList, stopperChan)
	kr.mu.Unlock()

	watchFunc := func(options metav1.ListOptions) (apiWatch.Interface, error) {
		options.FieldSelector = config.FieldSelector
		options.LabelSelector = config.LabelSelector
		return resource.Watch(ctx, options)
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	cfgCopy := *config
	wait.UntilWithContext(cancelCtx, func(newCtx context.Context) {
		resourceVersion, err := getResourceVersion(newCtx, &cfgCopy, resource)
		if err != nil {
			kr.setting.Logger.Error("could not retrieve a resourceVersion",
				zap.String("resource", cfgCopy.gvr.String()),
				zap.Error(err))
			cancel()
			return
		}

		done := kr.doWatch(newCtx, &cfgCopy, resourceVersion, watchFunc, stopperChan)
		if done {
			cancel()
			return
		}

		// need to restart with a fresh resource version
		cfgCopy.ResourceVersion = ""
	}, 0)
}

// doWatch returns true when watching is done, false when watching should be restarted.
func (kr *k8sobjectsreceiver) doWatch(ctx context.Context, config *K8sObjectsConfig, resourceVersion string, watchFunc func(options metav1.ListOptions) (apiWatch.Interface, error), stopperChan chan struct{}) bool {
	watcher, err := watch.NewRetryWatcher(resourceVersion, &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		kr.setting.Logger.Error("error in watching object",
			zap.String("resource", config.gvr.String()),
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
					kr.setting.Logger.Info("received a 410, grabbing new resource version",
						zap.Any("data", data))
					// we received a 410 so we need to restart
					return false
				}
			}

			if !ok {
				kr.setting.Logger.Warn("Watch channel closed unexpectedly",
					zap.String("resource", config.gvr.String()))
				return true
			}

			if config.exclude[data.Type] {
				kr.setting.Logger.Debug("dropping excluded data",
					zap.String("type", string(data.Type)))
				continue
			}

			logs, err := watchObjectsToLogData(&data, time.Now(), config)
			if err != nil {
				kr.setting.Logger.Error("error converting objects to log data", zap.Error(err))
			} else {
				obsCtx := kr.obsrecv.StartLogsOp(ctx)
				err := kr.consumer.ConsumeLogs(obsCtx, logs)
				kr.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), 1, err)
			}
		case <-stopperChan:
			watcher.Stop()
			return true
		}
	}
}

func getResourceVersion(ctx context.Context, config *K8sObjectsConfig, resource dynamic.ResourceInterface) (string, error) {
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
			return "", fmt.Errorf("could not perform initial list for watch on %v, %w", config.gvr.String(), err)
		}
		if objects == nil {
			return "", errors.New("nil objects returned, this is an error in the k8sobjectsreceiver")
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

// handleError handles errors according to the configured error mode
func (kr *k8sobjectsreceiver) handleError(err error, msg string) error {
	if err == nil {
		return nil
	}

	switch kr.config.ErrorMode {
	case PropagateError:
		if msg != "" {
			return fmt.Errorf("%s: %w", msg, err)
		}
		return err
	case IgnoreError:
		if msg != "" {
			kr.setting.Logger.Info(msg, zap.Error(err))
		} else {
			kr.setting.Logger.Info(err.Error())
		}
		return nil
	case SilentError:
		return nil
	default:
		// This shouldn't happen as we validate ErrorMode during config validation
		return fmt.Errorf("invalid error_mode %q: %w", kr.config.ErrorMode, err)
	}
}
