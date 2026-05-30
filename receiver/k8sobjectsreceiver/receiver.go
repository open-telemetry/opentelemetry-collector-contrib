// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
	informerobserver "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/informer"
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
	registry        *informerobserver.FactoryRegistry
	mu              sync.Mutex
	cancel          context.CancelFunc
	observerFunc    func(ctx context.Context, object *K8sObjectsConfig) (k8sinventory.Observer, error)
	wg              sync.WaitGroup
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
	}

	kr := &k8sobjectsreceiver{
		setting:  params,
		config:   config,
		objects:  objects,
		consumer: consumer,
		obsrecv:  obsrecv,
		mu:       sync.Mutex{},
	}

	kr.observerFunc = getObserverFunc(kr)

	return kr, nil
}

func getObserverFunc(kr *k8sobjectsreceiver) func(ctx context.Context, object *K8sObjectsConfig) (k8sinventory.Observer, error) {
	return func(ctx context.Context, object *K8sObjectsConfig) (k8sinventory.Observer, error) {
		obsConf := k8sinventory.Config{
			Gvr:           *object.gvr,
			Namespaces:    object.Namespaces,
			LabelSelector: object.LabelSelector,
			FieldSelector: object.FieldSelector,
		}

		switch object.Mode {
		case k8sinventory.PullMode:
			return informerobserver.NewPull(
				kr.registry,
				informerobserver.PullConfig{
					Config:           obsConf,
					Interval:         object.Interval,
					CacheSyncTimeout: kr.config.InformerCacheSyncTimeout,
				},
				kr.setting.Logger,
				func(objects *unstructured.UnstructuredList) {
					logs := pullObjectsToLogData(objects, time.Now(), object, kr.setting.BuildInfo.Version)
					obsCtx := kr.obsrecv.StartLogsOp(ctx)
					logRecordCount := logs.LogRecordCount()
					err := kr.consumer.ConsumeLogs(obsCtx, logs)
					kr.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), logRecordCount, err)
				},
			)
		case k8sinventory.WatchMode:
			return informerobserver.NewWatch(
				kr.registry,
				informerobserver.WatchConfig{
					Config:              obsConf,
					IncludeInitialState: kr.config.IncludeInitialState,
					Exclude:             object.exclude,
					CacheSyncTimeout:    kr.config.InformerCacheSyncTimeout,
				},
				kr.setting.Logger,
				func(data *apiWatch.Event) {
					logs, err := watchObjectsToLogData(data, time.Now(), object, kr.setting.BuildInfo.Version)
					if err != nil {
						kr.setting.Logger.Error("error converting objects to log data", zap.Error(err))
					} else {
						obsCtx := kr.obsrecv.StartLogsOp(ctx)
						err := kr.consumer.ConsumeLogs(obsCtx, logs)
						kr.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), 1, err)
					}
				},
			)
		}
		return nil, fmt.Errorf("invalid observer mode: %s", object.Mode)
	}
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

	for _, object := range validConfigs {
		if object.ResourceVersion != "" {
			kr.setting.Logger.Warn("resource_version is no longer used; the informer manages watch resumption internally")
			break
		}
	}
	if kr.config.IncludeInitialState {
		for _, object := range validConfigs {
			if object.Mode == k8sinventory.PullMode {
				kr.setting.Logger.Warn("include_initial_state is ignored in pull mode; pull mode always emits a full snapshot on the first tick")
				break
			}
		}
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

		// Register callbacks with the leader elector extension. These callbacks remain active
		// for the lifetime of the receiver, allowing it to restart when leadership is regained.
		elector.SetCallBackFuncs(
			func(ctx context.Context) {
				cctx, cancel := context.WithCancel(ctx)
				kr.cancel = cancel
				kr.registry = informerobserver.NewFactoryRegistry(kr.client)
				var (
					startWg sync.WaitGroup
					hasErr  bool
					errMu   sync.Mutex
				)
				for _, object := range validConfigs {
					startWg.Add(1)
					go func(obj *K8sObjectsConfig) {
						defer startWg.Done()
						if startErr := kr.start(cctx, obj); startErr != nil {
							kr.setting.Logger.Error("Could not start receiver for object type", zap.String("object", obj.Name), zap.Error(startErr))
							errMu.Lock()
							hasErr = true
							errMu.Unlock()
						}
					}(object)
				}
				startWg.Wait()
				if hasErr {
					cancel()
					return
				}
				kr.setting.Logger.Info("Object Receiver started as leader")
			},
			func() {
				// Shutdown on leader loss. The receiver will restart if leadership is regained
				// since the callbacks remain registered with the leader elector extension.
				kr.setting.Logger.Info("no longer leader, stopping")
				err = kr.Shutdown(context.Background())
				if err != nil {
					kr.setting.Logger.Error("shutdown receiver error:", zap.Error(err))
				}
			})
	} else {
		cctx, cancel := context.WithCancel(ctx)
		kr.cancel = cancel
		kr.registry = informerobserver.NewFactoryRegistry(kr.client)
		var (
			startWg  sync.WaitGroup
			firstErr error
			errMu    sync.Mutex
		)
		for _, object := range validConfigs {
			startWg.Add(1)
			go func(obj *K8sObjectsConfig) {
				defer startWg.Done()
				if startErr := kr.start(cctx, obj); startErr != nil {
					kr.setting.Logger.Error("failed to start observer for object", zap.String("object", obj.Name), zap.Error(startErr))
					errMu.Lock()
					if firstErr == nil {
						firstErr = startErr
					}
					errMu.Unlock()
				}
			}(object)
		}
		startWg.Wait()
		if firstErr != nil {
			cancel()
			return firstErr
		}
	}

	return nil
}

func (kr *k8sobjectsreceiver) Shutdown(ctx context.Context) error {
	kr.setting.Logger.Info("Object Receiver stopped")
	if kr.cancel != nil {
		kr.cancel()
	}
	kr.stopWatches()

	if kr.registry != nil {
		kr.registry.Shutdown()
		kr.registry = nil
	}

	return nil
}

func (kr *k8sobjectsreceiver) stopWatches() {
	kr.mu.Lock()
	for _, stopperChan := range kr.stopperChanList {
		close(stopperChan)
	}
	kr.mu.Unlock()
	kr.wg.Wait()
	kr.mu.Lock()
	kr.stopperChanList = nil
	kr.mu.Unlock()
}

func (kr *k8sobjectsreceiver) start(ctx context.Context, object *K8sObjectsConfig) error {
	if len(object.ExcludeNamespaces) > 0 {
		allNamespaces, err := kr.client.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}).List(ctx, metav1.ListOptions{})
		if err != nil {
			kr.setting.Logger.Error("failed to list namespaces", zap.Error(err))
			return err
		}
		compiledRegexes := make([]*regexp.Regexp, 0, len(object.ExcludeNamespaces))
		for _, pattern := range object.ExcludeNamespaces {
			re, err := regexp.Compile(pattern.Regex)
			if err != nil {
				kr.setting.Logger.Error("failed to compile regex "+pattern.Regex, zap.Error(err))
				continue
			}
			compiledRegexes = append(compiledRegexes, re)
		}
		// Rebuild each call to avoid duplicate namespaces on re-election.
		var includedNamespaces []string
		for _, ns := range allNamespaces.Items {
			excluded := false
			for _, re := range compiledRegexes {
				if re.MatchString(ns.GetName()) {
					excluded = true
					break
				}
			}
			if !excluded {
				includedNamespaces = append(includedNamespaces, ns.GetName())
			}
		}
		object.Namespaces = includedNamespaces

		kr.setting.Logger.Info("Collecting from namespaces", zap.Strings("namespaces", object.Namespaces))
	}

	obs, err := kr.observerFunc(ctx, object)
	if err != nil {
		return err
	}

	stopChan, err := obs.Start(ctx, &kr.wg)
	if err != nil {
		return err
	}
	kr.mu.Lock()
	kr.stopperChanList = append(kr.stopperChanList, stopChan)
	kr.mu.Unlock()

	return nil
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
