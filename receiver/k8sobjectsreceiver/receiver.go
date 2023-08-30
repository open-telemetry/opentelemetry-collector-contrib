// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/internal/metadata"
)

type k8sobjectsreceiver struct {
	setting         receiver.CreateSettings
	objects         []*K8sObjectsConfig
	stopperChanList []chan struct{}
	client          dynamic.Interface
	consumer        consumer.Logs
	obsrecv         *obsreport.Receiver
	mu              sync.Mutex
}

func newReceiver(params receiver.CreateSettings, config *Config, consumer consumer.Logs) (receiver.Logs, error) {
	transport := "http"
	client, err := config.getDynamicClient()
	if err != nil {
		return nil, err
	}

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
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
		client:   client,
		setting:  params,
		consumer: consumer,
		objects:  config.Objects,
		obsrecv:  obsrecv,
		mu:       sync.Mutex{},
	}, nil
}

func (kr *k8sobjectsreceiver) Start(ctx context.Context, _ component.Host) error {
	kr.setting.Logger.Info("Object Receiver started")

	for _, object := range kr.objects {
		kr.start(ctx, object)
	}
	return nil
}

func (kr *k8sobjectsreceiver) Shutdown(context.Context) error {
	kr.setting.Logger.Info("Object Receiver stopped")
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
	ticker := newTicker(config.Interval)
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
				kr.setting.Logger.Error("error in pulling object", zap.String("resource", config.gvr.String()), zap.Error(err))
			} else if len(objects.Items) > 0 {
				logs := pullObjectsToLogData(objects, time.Now(), config)
				obsCtx := kr.obsrecv.StartLogsOp(ctx)
				err = kr.consumer.ConsumeLogs(obsCtx, logs)
				kr.obsrecv.EndLogsOp(obsCtx, metadata.Type, logs.LogRecordCount(), err)
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

	resourceVersion, err := getResourceVersion(ctx, config, resource)
	if err != nil {
		kr.setting.Logger.Error("could not retrieve an initial resourceVersion", zap.String("resource", config.gvr.String()), zap.Error(err))
		return
	}
	watchFunc := func(options metav1.ListOptions) (apiWatch.Interface, error) {
		options.FieldSelector = config.FieldSelector
		options.LabelSelector = config.LabelSelector
		return resource.Watch(ctx, options)
	}

	watcher, err := watch.NewRetryWatcher(resourceVersion, &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		kr.setting.Logger.Error("error in watching object", zap.String("resource", config.gvr.String()), zap.Error(err))
		return
	}

	res := watcher.ResultChan()
	for {
		select {
		case data, ok := <-res:
			if !ok {
				kr.setting.Logger.Warn("Watch channel closed unexpectedly", zap.String("resource", config.gvr.String()))
				return
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
				kr.obsrecv.EndLogsOp(obsCtx, metadata.Type, 1, err)
			}
		case <-stopperChan:
			watcher.Stop()
			return
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
			return "", fmt.Errorf("nil objects returned, this is an error in the k8sobjectsreceiver")
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
func newTicker(repeat time.Duration) *time.Ticker {
	ticker := time.NewTicker(repeat)
	oc := ticker.C
	nc := make(chan time.Time, 1)
	go func() {
		nc <- time.Now()
		for tm := range oc {
			nc <- tm
		}
	}()
	ticker.C = nc
	return ticker
}
