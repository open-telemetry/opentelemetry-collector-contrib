// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/metadata"
)

type k8seventsReceiver struct {
	config          *Config
	settings        receiver.CreateSettings
	logsConsumer    consumer.Logs
	stopperChanList []chan struct{}
	startTime       time.Time
	ctx             context.Context
	cancel          context.CancelFunc
	obsrecv         *receiverhelper.ObsReport
}

// newReceiver creates the Kubernetes events receiver with the given configuration.
func newReceiver(
	set receiver.CreateSettings,
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

func (kr *k8seventsReceiver) Start(ctx context.Context, _ component.Host) error {
	kr.ctx, kr.cancel = context.WithCancel(ctx)

	k8sInterface, err := kr.config.getK8sClient()
	if err != nil {
		return err
	}

	kr.settings.Logger.Info("starting to watch namespaces for the events.")
	if len(kr.config.Namespaces) == 0 {
		kr.startWatch(corev1.NamespaceAll, k8sInterface)
	} else {
		for _, ns := range kr.config.Namespaces {
			kr.startWatch(ns, k8sInterface)
		}
	}

	return nil
}

func (kr *k8seventsReceiver) Shutdown(context.Context) error {
	if kr.cancel == nil {
		return nil
	}
	// Stop watching all the namespaces by closing all the stopper channels.
	for _, stopperChan := range kr.stopperChanList {
		close(stopperChan)
	}
	kr.cancel()
	return nil
}

// Add the 'Event' handler and trigger the watch for a specific namespace.
// For new and updated events, the code is relying on the following k8s code implementation:
// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/client-go/tools/record/events_cache.go#L327
func (kr *k8seventsReceiver) startWatch(ns string, client k8s.Interface) {
	stopperChan := make(chan struct{})
	kr.stopperChanList = append(kr.stopperChanList, stopperChan)
	kr.startWatchingNamespace(client, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			ev := obj.(*corev1.Event)
			kr.handleEvent(ev)
		},
		UpdateFunc: func(_, obj any) {
			ev := obj.(*corev1.Event)
			kr.handleEvent(ev)
		},
	}, ns, stopperChan)
}

func (kr *k8seventsReceiver) handleEvent(ev *corev1.Event) {
	if kr.allowEvent(ev) {
		ld := k8sEventToLogData(kr.settings.Logger, ev)

		ctx := kr.obsrecv.StartLogsOp(kr.ctx)
		consumerErr := kr.logsConsumer.ConsumeLogs(ctx, ld)
		kr.obsrecv.EndLogsOp(ctx, metadata.Type, 1, consumerErr)
	}
}

// startWatchingNamespace creates an informer and starts
// watching a specific namespace for the events.
func (kr *k8seventsReceiver) startWatchingNamespace(
	clientset k8s.Interface,
	handlers cache.ResourceEventHandlerFuncs,
	ns string,
	stopper chan struct{},
) {
	client := clientset.CoreV1().RESTClient()
	watchList := cache.NewListWatchFromClient(client, "events", ns, fields.Everything())
	_, controller := cache.NewInformer(watchList, &corev1.Event{}, 0, handlers)
	go controller.Run(stopper)
}

// Allow events with eventTimestamp(EventTime/LastTimestamp/FirstTimestamp)
// not older than the receiver start time so that
// event flood can be avoided upon startup.
func (kr *k8seventsReceiver) allowEvent(ev *corev1.Event) bool {
	eventTimestamp := getEventTimestamp(ev)
	return !eventTimestamp.Before(kr.startTime)
}

// Return the EventTimestamp based on the populated k8s event timestamps.
// Priority: EventTime > LastTimestamp > FirstTimestamp.
func getEventTimestamp(ev *corev1.Event) time.Time {
	var eventTimestamp time.Time

	switch {
	case ev.EventTime.Time != time.Time{}:
		eventTimestamp = ev.EventTime.Time
	case ev.LastTimestamp.Time != time.Time{}:
		eventTimestamp = ev.LastTimestamp.Time
	case ev.FirstTimestamp.Time != time.Time{}:
		eventTimestamp = ev.FirstTimestamp.Time
	}

	return eventTimestamp
}
