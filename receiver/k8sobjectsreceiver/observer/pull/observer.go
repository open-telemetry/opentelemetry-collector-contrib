package pull

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/observer"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sync"
	"time"
)

type Observer struct {
	config observer.Config

	client dynamic.Interface
	logger *zap.Logger
	mu     sync.Mutex

	handlePullObjectsFunc func(objects *unstructured.UnstructuredList)
}

func New(client dynamic.Interface, config observer.Config, logger *zap.Logger, handlePullObjectsFunc func(objects *unstructured.UnstructuredList)) (*Observer, error) {
	o := &Observer{
		client:                client,
		config:                config,
		logger:                logger,
		handlePullObjectsFunc: handlePullObjectsFunc,
	}
	return o, nil
}

func (o *Observer) Start(ctx context.Context) chan struct{} {
	resource := o.client.Resource(o.config.Gvr)
	o.logger.Info("Started collecting",
		zap.Any("gvr", o.config.Gvr),
		zap.Any("mode", "pull"),
		zap.Any("namespaces", o.config.Namespaces))

	stopperChan := make(chan struct{})

	if len(o.config.Namespaces) == 0 {
		go o.startPull(ctx, resource, stopperChan)
	} else {
		for _, ns := range o.config.Namespaces {
			go o.startPull(ctx, resource.Namespace(ns), stopperChan)
		}
	}

	return stopperChan
}

func (o *Observer) startPull(ctx context.Context, resource dynamic.ResourceInterface, stopperChan chan struct{}) {
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
				if o.handlePullObjectsFunc != nil {
					o.handlePullObjectsFunc(objects)
				}
			}
		case <-stopperChan:
			return
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
