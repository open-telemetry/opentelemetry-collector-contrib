// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"context"
	"net/http"
	"strings"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/internal/metadata"
)

//need a semaphore kind of variable to wait watch for all the goroutines of length (object count)

type k8sobjectsreceiver struct {
	setting         receiver.Settings
	config          *Config
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

	for _, object := range config.Objects {
		object.exclude = make(map[apiWatch.EventType]bool)
		for _, item := range object.ExcludeWatchType {
			object.exclude[item] = true
		}
	}

	return &k8sobjectsreceiver{
		setting:  params,
		consumer: consumer,
		config:   config,
		obsrecv:  obsrecv,
		mu:       sync.Mutex{},
	}, nil
}

func (kr *k8sobjectsreceiver) Start(ctx context.Context, _ component.Host) error {
	client, err := kr.config.getDynamicClient()
	if err != nil {
		return err
	}
	kr.client = client
	kr.setting.Logger.Info("Object Receiver started")

	cctx, cancel := context.WithCancel(ctx)
	kr.cancel = cancel

	var listWatchObjects int
	var listWatchInterval time.Duration
	for _, object := range kr.config.Objects {
		if object.Mode == ListWatchMode {
			listWatchObjects++
			if strings.ToLower(object.Name) == "pods" {
				listWatchInterval = object.Interval
			}
		} else {
			kr.start(cctx, object)
		}
	}
	if listWatchObjects > 0 {
		//if there is no list watch interval set for pods, use the first object interval
		if listWatchInterval == 0 {
			listWatchInterval = kr.config.Objects[0].Interval
		}
		go kr.startListWatchObjects(cctx, kr.config.Objects, listWatchInterval)
	}
	return nil
}

func (kr *k8sobjectsreceiver) startListWatchObjects(ctx context.Context, objects []*K8sObjectsConfig, interval time.Duration) {
	stopperChan := make(chan struct{})
	kr.mu.Lock()
	kr.stopperChanList = append(kr.stopperChanList, stopperChan)
	kr.mu.Unlock()

	// Start a ticker for the list watch mode
	ticker := newTicker(ctx, interval)
	defer ticker.Stop()

	stopperChanNew := make(chan struct{})
	cancelCtx, cancel := context.WithCancel(ctx)

	for {
		select {
		case <-ticker.C:
			if stopperChanNew != nil {
				close(stopperChanNew)
			}
			cancel()
			stopperChanNew = make(chan struct{})
			cancelCtx, cancel = context.WithCancel(ctx)
			pullBarrier := make(chan struct{})

			// Create a new WaitGroup for each cycle
			var pullWQ sync.WaitGroup

			for _, object := range objects {
				if object.Mode == ListWatchMode {
					resource := kr.client.Resource(*object.gvr)
					kr.setting.Logger.Info("Started collecting", zap.Any("gvr", object.gvr), zap.Any("mode", object.Mode), zap.Any("namespaces", object.Namespaces))
					if len(object.Namespaces) == 0 {
						pullWQ.Add(1)
						go kr.startListWatch(cancelCtx, object, resource, stopperChanNew, cancel, &pullWQ, pullBarrier)
					} else {
						for _, ns := range object.Namespaces {
							pullWQ.Add(1)
							go kr.startListWatch(cancelCtx, object, resource.Namespace(ns), stopperChanNew, cancel, &pullWQ, pullBarrier)
						}
					}
				}
			}
			kr.setting.Logger.Info("timer waiting for all pull operations to finish before starting watch")
			pullWQ.Wait()
			kr.setting.Logger.Info("timer waiting over for all pull operations and starting watch")
			//send a final log for the end of the pull operation
			pullEndLog := createResourcePullEndLog(nil)
			obsCtx := kr.obsrecv.StartLogsOp(ctx)
			err := kr.consumer.ConsumeLogs(obsCtx, pullEndLog)
			kr.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), pullEndLog.LogRecordCount(), err)
			//Notify to start watch
			close(pullBarrier)
		case <-stopperChan:
			cancel()
			if stopperChanNew != nil {
				close(stopperChanNew)
			}
			return
		}
	}

}

func (kr *k8sobjectsreceiver) startListWatch(ctx context.Context, config *K8sObjectsConfig, resource dynamic.ResourceInterface, stopperChanNew chan struct{}, cancel context.CancelFunc, pullWQ *sync.WaitGroup, pullBarrier chan struct{}) {

	watchFunc := func(options metav1.ListOptions) (apiWatch.Interface, error) {
		options.FieldSelector = config.FieldSelector
		options.LabelSelector = config.LabelSelector
		return resource.Watch(ctx, options)
	}
	go kr.pullAndDoWatch(ctx, *config, resource, cancel, watchFunc, stopperChanNew, pullWQ, pullBarrier)

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
	kr.setting.Logger.Info("Started collecting", zap.Any("gvr", object.gvr), zap.Any("mode", object.Mode), zap.Any("namespaces", object.Namespaces))

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
			continueToken := ""
			pageLimit := config.PageLimit
			if pageLimit <= 0 {
				pageLimit = 500
			}
			for {
				listOption.Limit = int64(pageLimit)
				listOption.Continue = continueToken
				objects, err := resource.List(ctx, listOption)
				if err != nil {
					kr.setting.Logger.Error("error in pulling object", zap.String("resource", config.gvr.String()), zap.Error(err))
					break
				} else if len(objects.Items) > 0 {
					logs := pullObjectsToLogData(objects, time.Now(), config)
					obsCtx := kr.obsrecv.StartLogsOp(ctx)
					logRecordCount := logs.LogRecordCount()
					err = kr.consumer.ConsumeLogs(obsCtx, logs)
					kr.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), logRecordCount, err)
				}
				continueToken = objects.GetContinue()
				if continueToken == "" {
					break
				}
				time.Sleep(config.PageInterval)
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
	defer cancel()
	cfgCopy := *config
	kr.pullAndDoWatch(cancelCtx, cfgCopy, resource, cancel, watchFunc, stopperChan, nil, nil)
}

func (kr *k8sobjectsreceiver) pullAndDoWatch(cancelCtx context.Context, cfgCopy K8sObjectsConfig, resource dynamic.ResourceInterface, cancel context.CancelFunc, watchFunc func(options metav1.ListOptions) (apiWatch.Interface, error), stopperChan chan struct{}, pullWQ *sync.WaitGroup, pullBarrier chan struct{}) {
	wait.UntilWithContext(cancelCtx, func(newCtx context.Context) {
		resourceVersion, err := kr.doPullOnceAndGetResourceVersion(newCtx, &cfgCopy, resource)
		if pullWQ != nil {
			pullWQ.Done()
		}
		if err != nil {
			kr.setting.Logger.Error("could not retrieve a resourceVersion", zap.String("resource", cfgCopy.gvr.String()), zap.Error(err))
			cancel()
			return
		}
		if pullBarrier != nil {
			// Wait for all threads to finish the pull operation
			kr.setting.Logger.Debug("Waiting for all pull operations to finish before starting watch", zap.String("resource", cfgCopy.gvr.String()))
			<-pullBarrier
			kr.setting.Logger.Debug("Waiting over for all pull operations and starting watch", zap.String("resource", cfgCopy.gvr.String()))
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
		kr.setting.Logger.Error("error in watching object", zap.String("resource", config.gvr.String()), zap.Error(err))
		return true
	}

	defer watcher.Stop()
	res := watcher.ResultChan()
	for {
		select {
		case data, ok := <-res:
			if data.Type == apiWatch.Error {
				errObject := apierrors.FromObject(data.Object)
				// nolint:errorlint
				if errObject.(*apierrors.StatusError).ErrStatus.Code == http.StatusGone {
					kr.setting.Logger.Info("received a 410, grabbing new resource version", zap.Any("data", data))
					// we received a 410 so we need to restart
					return false
				}
			}

			if !ok {
				kr.setting.Logger.Warn("Watch channel closed unexpectedly", zap.String("resource", config.gvr.String()))
				return true
			}

			if config.exclude[data.Type] {
				kr.setting.Logger.Debug("dropping excluded data", zap.String("type", string(data.Type)))
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

func (kr *k8sobjectsreceiver) doPullOnceAndGetResourceVersion(ctx context.Context, config *K8sObjectsConfig, resource dynamic.ResourceInterface) (string, error) {
	resourceVersion := config.ResourceVersion
	if resourceVersion == "" || resourceVersion == "0" {
		// Proper use of the Kubernetes API Watch capability when no resourceVersion is supplied is to do a list first
		// to get the initial state and a useable resourceVersion.
		// See https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes for details.

		listOption := metav1.ListOptions{
			FieldSelector: config.FieldSelector,
			LabelSelector: config.LabelSelector,
		}

		resourceVersion = kr.resourcePullWithPagination(ctx, config, resource, listOption)

		// If we still don't have a resourceVersion we can try 1 as a last ditch effort.
		// This also helps our unit tests since the fake client can't handle returning resource versions
		// as part of a list of objects.
		if resourceVersion == "" || resourceVersion == "0" {
			resourceVersion = defaultResourceVersion
		}

		// In case of ListWatch mode, log even the initial list output.
		if config.Mode == ListWatchMode {
			if config.Interval != 0 {

			}
		}
	}
	return resourceVersion, nil
}

func (kr *k8sobjectsreceiver) startPeriodicList(ctx context.Context, config *K8sObjectsConfig, resource dynamic.ResourceInterface) {
	stopperChan := make(chan struct{})
	kr.mu.Lock()
	kr.stopperChanList = append(kr.stopperChanList, stopperChan)
	kr.mu.Unlock()
	ticker := time.NewTicker(config.Interval)
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
			kr.resourcePullWithPagination(ctx, config, resource, listOption)
		case <-stopperChan:
			return
		}

	}
}

func (kr *k8sobjectsreceiver) resourcePullWithPagination(ctx context.Context, config *K8sObjectsConfig, resource dynamic.ResourceInterface, listOption metav1.ListOptions) string {
	if config.PageLimit <= 0 {
		config.PageLimit = 500
	}
	continueToken := ""
	resourceVersion := ""
	var pageCount int
	for {
		listOption.Limit = int64(config.PageLimit)
		listOption.Continue = continueToken
		objects, err := resource.List(ctx, listOption)
		if err != nil {
			kr.setting.Logger.Error("error in pulling object", zap.String("resource", config.gvr.String()), zap.Error(err))
			break
		} else if len(objects.Items) > 0 {
			if pageCount == 0 {
				resourceVersion = objects.GetResourceVersion()
				if resourceVersion == "" || resourceVersion == "0" {
					resourceVersion = defaultResourceVersion
				}
			}
			pageCount++
			logs := pullObjectsToLogData(objects, time.Now(), config)
			obsCtx := kr.obsrecv.StartLogsOp(ctx)
			logRecordCount := logs.LogRecordCount()
			err = kr.consumer.ConsumeLogs(obsCtx, logs)
			kr.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), logRecordCount, err)
		}
		continueToken = objects.GetContinue()
		if continueToken == "" {
			//send one watch event for the end of the list with type "PullEnd" and kind in the object should be the config.name, send this as log
			pullEndLog := createResourcePullEndLog(config)
			obsCtx := kr.obsrecv.StartLogsOp(ctx)
			err = kr.consumer.ConsumeLogs(obsCtx, pullEndLog)
			kr.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), pullEndLog.LogRecordCount(), err)
			break
		}
	}
	return resourceVersion
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
