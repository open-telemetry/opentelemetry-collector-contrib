// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

var _ cache.ResourceEventHandler = (*handler)(nil)

type handler struct {
	endpoints *sync.Map
	callback  func(ctx context.Context) ([]string, error)
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder
}

func (h handler) OnAdd(obj any, _ bool) {
	var endpoints []string

	switch object := obj.(type) {
	case *corev1.Endpoints:
		endpoints = convertToEndpoints(object)
	default: // unsupported
		h.logger.Warn("Got an unexpected Kubernetes data type during the inclusion of a new pods for the service", zap.Any("obj", obj))
		h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
		return
	}
	changed := false
	for _, ep := range endpoints {
		if _, loaded := h.endpoints.LoadOrStore(ep, true); !loaded {
			changed = true
		}
	}
	if changed {
		_, _ = h.callback(context.Background())
	}
}

func (h handler) OnUpdate(oldObj, newObj any) {
	switch oldEps := oldObj.(type) {
	case *corev1.Endpoints:
		epRemove := convertToEndpoints(oldEps)
		for _, ep := range epRemove {
			h.endpoints.Delete(ep)
		}
		if len(epRemove) > 0 {
			_, _ = h.callback(context.Background())
		}

		newEps, ok := newObj.(*corev1.Endpoints)
		if !ok {
			h.logger.Warn("Got an unexpected Kubernetes data type during the update of the pods for a service", zap.Any("obj", newObj))
			h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
			return
		}
		changed := false
		for _, ep := range convertToEndpoints(newEps) {
			if _, loaded := h.endpoints.LoadOrStore(ep, true); !loaded {
				changed = true
			}
		}
		if changed {
			_, _ = h.callback(context.Background())
		}
	default: // unsupported
		h.logger.Warn("Got an unexpected Kubernetes data type during the update of the pods for a service", zap.Any("obj", oldObj))
		h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
		return
	}
}

func (h handler) OnDelete(obj any) {
	var endpoints []string
	switch object := obj.(type) {
	case *cache.DeletedFinalStateUnknown:
		h.OnDelete(object.Obj)
		return
	case *corev1.Endpoints:
		if object != nil {
			endpoints = convertToEndpoints(object)
		}
	default: // unsupported
		h.logger.Warn("Got an unexpected Kubernetes data type during the removal of the pods for a service", zap.Any("obj", obj))
		h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
		return
	}
	if len(endpoints) != 0 {
		for _, endpoint := range endpoints {
			h.endpoints.Delete(endpoint)
		}
		_, _ = h.callback(context.Background())
	}
}

func convertToEndpoints(eps ...*corev1.Endpoints) []string {
	var ipAddress []string
	for _, ep := range eps {
		for _, subsets := range ep.Subsets {
			for _, addr := range subsets.Addresses {
				ipAddress = append(ipAddress, addr.IP)
			}
		}
	}
	return ipAddress
}
