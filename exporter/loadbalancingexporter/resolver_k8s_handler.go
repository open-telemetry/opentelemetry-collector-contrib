// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"slices"
	"sync"

	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

var _ cache.ResourceEventHandler = (*handler)(nil)

const (
	epMissingHostnamesMsg = "Endpoints object missing hostnames"
)

type handler struct {
	endpoints   *sync.Map
	callback    func(ctx context.Context) ([]string, error)
	logger      *zap.Logger
	telemetry   *metadata.TelemetryBuilder
	returnNames bool
	ports       []int32
}

func (h handler) OnAdd(obj any, _ bool) {
	var endpoints map[string]bool
	var ok bool

	switch object := obj.(type) {
	case *discoveryv1.EndpointSlice:
		ok, endpoints = convertEndpointSlices(object, h.ports)
		if !ok {
			h.logger.Warn(epMissingHostnamesMsg, zap.Any("obj", obj))
			h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
			return

		}
	case *corev1.Endpoints:
		ok, endpoints = convertToEndpoints(h.returnNames, object)
		if !ok {
			h.logger.Warn(epMissingHostnamesMsg, zap.Any("obj", obj))
			h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
			return
		}

	default: // unsupported
		h.logger.Warn("Got an unexpected Kubernetes data type during the inclusion of a new pods for the service", zap.Any("obj", obj))
		h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
		return
	}
	changed := false
	for ep := range endpoints {
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
		newEps, ok := newObj.(*corev1.Endpoints)
		if !ok {
			h.logger.Warn("Got an unexpected Kubernetes data type during the update of the pods for a service", zap.Any("obj", newObj))
			h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
			return
		}

		_, oldEndpoints := convertToEndpoints(h.returnNames, oldEps)
		hostnameOk, newEndpoints := convertToEndpoints(h.returnNames, newEps)
		if !hostnameOk {
			h.logger.Warn(epMissingHostnamesMsg, zap.Any("obj", newEps))
			h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
			return
		}

		changed := false

		// Iterate through old endpoints and remove those that are not in the new list.
		for ep := range oldEndpoints {
			if _, ok := newEndpoints[ep]; !ok {
				h.endpoints.Delete(ep)
				changed = true
			}
		}

		// Iterate through new endpoints and add those that are not in the endpoints map already.
		for ep := range newEndpoints {
			if _, loaded := h.endpoints.LoadOrStore(ep, true); !loaded {
				changed = true
			}
		}

		if changed {
			_, _ = h.callback(context.Background())
		} else {
			h.logger.Debug("No changes detected in the endpoints for the service", zap.Any("old", oldEps), zap.Any("new", newEps))
		}

	default: // unsupported
		h.logger.Warn("Got an unexpected Kubernetes data type during the update of the pods for a service", zap.Any("obj", oldObj))
		h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
		return
	}
}

func (h handler) OnDelete(obj any) {
	var endpoints map[string]bool
	var ok bool

	switch object := obj.(type) {
	case *cache.DeletedFinalStateUnknown:
		h.OnDelete(object.Obj)
		return
	case *corev1.Endpoints:
		if object != nil {
			ok, endpoints = convertToEndpoints(h.returnNames, object)
			if !ok {
				h.logger.Warn(epMissingHostnamesMsg, zap.Any("obj", obj))
				h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
				return
			}
		}
	default: // unsupported
		h.logger.Warn("Got an unexpected Kubernetes data type during the removal of the pods for a service", zap.Any("obj", obj))
		h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
		return
	}
	if len(endpoints) != 0 {
		for endpoint := range endpoints {
			h.endpoints.Delete(endpoint)
		}
		_, _ = h.callback(context.Background())
	}
}

func convertToEndpoints(retNames bool, eps ...*corev1.Endpoints) (bool, map[string]bool) {
	res := map[string]bool{}
	for _, ep := range eps {
		for _, subsets := range ep.Subsets {
			for _, addr := range subsets.Addresses {
				if retNames {
					if addr.Hostname == "" {
						return false, nil
					}
					res[addr.Hostname] = true
				} else {
					res[addr.IP] = true
				}
			}
		}
	}
	return true, res
}

func convertEndpointSlices(slice *discoveryv1.EndpointSlice, ports []int32) (bool, map[string]bool) {
	res := map[string]bool{}

	for _, port := range slice.Ports {
		if slices.Contains(ports, *port.Port) {
			for _, ep := range slice.Endpoints {
				for _, addr := range ep.Addresses {
					// TODO: support FQDN and IPv6?
					if slice.AddressType == discoveryv1.AddressTypeIPv4 {
						res[addr] = true
					}
				}
			}
		}
	}

	return true, res
}
