// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
	watchobserver "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/watch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/metadata"
)

type k8seventsReceiver struct {
	config          *Config
	settings        receiver.Settings
	logsConsumer    consumer.Logs
	stopperChanList []chan struct{}
	startTime       time.Time
	ctx             context.Context
	cancel          context.CancelFunc
	obsrecv         *receiverhelper.ObsReport
	mu              sync.Mutex
	client          dynamic.Interface
	wg              sync.WaitGroup
}

// newReceiver creates the Kubernetes events receiver with the given configuration.
func newReceiver(
	set receiver.Settings,
	config *Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	transport := "http"

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              transport,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	return &k8seventsReceiver{
		settings:     set,
		config:       config,
		logsConsumer: consumer,
		startTime:    time.Now(),
		obsrecv:      obsrecv,
	}, nil
}

func (kr *k8seventsReceiver) Start(ctx context.Context, host component.Host) error {
	kr.ctx, kr.cancel = context.WithCancel(ctx)

	client, err := kr.config.getDynamicClient()
	if err != nil {
		return err
	}
	kr.client = client

	if kr.config.K8sLeaderElector != nil {
		k8sLeaderElector := host.GetExtensions()[*kr.config.K8sLeaderElector]
		if k8sLeaderElector == nil {
			return fmt.Errorf("unknown k8s leader elector %q", kr.config.K8sLeaderElector)
		}

		elector, ok := k8sLeaderElector.(k8sleaderelector.LeaderElection)
		if !ok {
			return fmt.Errorf("the extension %T does not implement k8sleaderelector.LeaderElection", k8sLeaderElector)
		}

		kr.settings.Logger.Info("registering the receiver in leader election")

		// Register callbacks with the leader elector extension. These callbacks remain active
		// for the lifetime of the receiver, allowing it to restart when leadership is regained.
		elector.SetCallBackFuncs(
			func(ctx context.Context) {
				cctx, cancel := context.WithCancel(ctx)
				kr.cancel = cancel
				kr.ctx = cctx
				kr.settings.Logger.Info("Events Receiver started as leader")
				kr.startWatchers()
			},
			func() {
				// Shutdown on leader loss. The receiver will restart if leadership is regained
				// since the callbacks remain registered with the leader elector extension.
				kr.settings.Logger.Info("no longer leader, stopping")
				err := kr.Shutdown(context.Background())
				if err != nil {
					kr.settings.Logger.Error("shutdown receiver error:", zap.Error(err))
				}
			})
		return nil
	}

	// No leader election: start immediately.
	kr.settings.Logger.Info("starting to watch namespaces for the events.")
	kr.startWatchers()
	return nil
}

func (kr *k8seventsReceiver) Shutdown(context.Context) error {
	if kr.cancel != nil {
		kr.cancel()
	}

	kr.mu.Lock()
	for _, stopperChan := range kr.stopperChanList {
		close(stopperChan)
	}
	kr.stopperChanList = nil
	kr.mu.Unlock()
	kr.wg.Wait()
	return nil
}

// startWatchers creates and starts the k8sinventory watch observer
func (kr *k8seventsReceiver) startWatchers() {
	// Events GVR (GroupVersionResource)
	eventsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "events",
	}

	namespaces := kr.config.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""} // Empty string means all namespaces
	}

	observer, err := watchobserver.New(
		kr.client,
		watchobserver.Config{
			Config: k8sinventory.Config{
				Gvr:        eventsGVR,
				Namespaces: namespaces,
			},
			IncludeInitialState: false, // Don't send initial state, only new events
			Exclude:             nil,   // Don't exclude any event types
		},
		kr.settings.Logger,
		func(event *apiWatch.Event) {
			// The k8sinventory watch observer uses dynamic client which returns unstructured objects
			// We need to convert them to corev1.Event
			unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				kr.settings.Logger.Warn("Received non-Unstructured object from watch",
					zap.String("type", fmt.Sprintf("%T", event.Object)))
				return
			}

			// Convert unstructured to corev1.Event
			ev := &corev1.Event{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, ev)
			if err != nil {
				kr.settings.Logger.Error("Failed to convert unstructured object to Event",
					zap.Error(err))
				return
			}

			kr.handleEvent(ev)
		},
	)
	if err != nil {
		kr.settings.Logger.Error("Failed to create watch observer", zap.Error(err))
		return
	}

	stopperChan := observer.Start(kr.ctx, &kr.wg)
	kr.mu.Lock()
	kr.stopperChanList = append(kr.stopperChanList, stopperChan)
	kr.mu.Unlock()
}

// handleEvent processes a Kubernetes event and sends it to the logs consumer
func (kr *k8seventsReceiver) handleEvent(ev *corev1.Event) {
	if kr.allowEvent(ev) {
		ld := k8sEventToLogData(kr.settings.Logger, ev, kr.settings.BuildInfo.Version)

		ctx := kr.obsrecv.StartLogsOp(kr.ctx)
		consumerErr := kr.logsConsumer.ConsumeLogs(ctx, ld)
		kr.obsrecv.EndLogsOp(ctx, metadata.Type.String(), 1, consumerErr)
	}
}

// Allow events with eventTimestamp(EventTime/LastTimestamp/FirstTimestamp)
// not older than the receiver start time so that
// event flood can be avoided upon startup.
func (kr *k8seventsReceiver) allowEvent(ev *corev1.Event) bool {
	eventTimestamp := k8sinventory.GetEventTimestamp(ev)
	return !eventTimestamp.Before(kr.startTime)
}
