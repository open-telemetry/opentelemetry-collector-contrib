// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

var _ cache.ResourceEventHandler = (*handler)(nil)

const (
	epMissingHostnamesMsg = "EndpointSlice object missing hostnames"
)

type handler struct {
	endpoints   *sync.Map
	callback    func(ctx context.Context) ([]string, error)
	logger      *zap.Logger
	telemetry   *metadata.TelemetryBuilder
	returnNames bool
}

func (h handler) OnAdd(obj any, _ bool) {
	var endpoints map[string]bool
	var ok bool

	switch object := obj.(type) {
	case *discoveryv1.EndpointSlice:
		ok, endpoints = convertToEndpoints(h.returnNames, object)
		if !ok {
			// Some endpoints were missing hostnames and were skipped. Still add
			// the ones we could resolve; the rest are picked up on a later event
			// once their hostname is populated.
			h.logger.Warn(epMissingHostnamesMsg, zap.Any("obj", obj))
			h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
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
	case *discoveryv1.EndpointSlice:
		newEps, ok := newObj.(*discoveryv1.EndpointSlice)
		if !ok {
			h.logger.Warn("Got an unexpected Kubernetes data type during the update of the pods for a service", zap.Any("obj", newObj))
			h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
			return
		}

		_, oldEndpoints := convertToEndpoints(h.returnNames, oldEps)
		newOk, newEndpoints := convertToEndpoints(h.returnNames, newEps)
		if !newOk {
			// The new slice has endpoint(s) without a hostname yet. Those are
			// skipped and picked up on a later event once their hostname is
			// populated. We still process the endpoints we can resolve (below)
			// instead of returning early, so pods that churned out are removed
			// and surviving pods are kept -- returning here previously stranded
			// dead pod hostnames in the store forever. Only the new slice's
			// validity drives the warning/metric, so recovering from a transient
			// missing-hostname state is not reported as a resolution failure.
			h.logger.Warn(epMissingHostnamesMsg, zap.Any("obj", newEps))
			h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
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
	case *discoveryv1.EndpointSlice:
		if object != nil {
			ok, endpoints = convertToEndpoints(h.returnNames, object)
			if !ok {
				// Some endpoints lacked hostnames (and so were never tracked);
				// still remove the ones we can resolve.
				h.logger.Warn(epMissingHostnamesMsg, zap.Any("obj", obj))
				h.telemetry.LoadbalancerNumResolutions.Add(context.Background(), 1, metric.WithAttributeSet(k8sResolverFailureAttrSet))
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

// convertToEndpoints extracts the set of routable endpoints from the given
// EndpointSlices: pod hostnames when retNames is true, otherwise IP addresses.
//
// The returned bool reports whether every endpoint could be converted. When
// retNames is true, any endpoint that does not (yet) have a hostname is skipped
// and the bool is false, but the endpoints that WERE converted are still
// returned. Callers must process that partial set rather than discarding it: a
// pod frequently appears in an EndpointSlice a moment before its Hostname field
// is populated during a rollout, and dropping the whole slice in that window
// meant pods that churned out were never removed from the resolver's store,
// leaking dead pod hostnames without bound.
func convertToEndpoints(retNames bool, eps ...*discoveryv1.EndpointSlice) (bool, map[string]bool) {
	res := map[string]bool{}
	ok := true
	for _, ep := range eps {
		for _, endpoint := range ep.Endpoints {
			for _, addr := range endpoint.Addresses {
				if retNames {
					if endpoint.Hostname == nil || *endpoint.Hostname == "" {
						ok = false
						continue
					}
					res[*endpoint.Hostname] = true
				} else {
					res[addr] = true
				}
			}
		}
	}
	return ok, res
}
