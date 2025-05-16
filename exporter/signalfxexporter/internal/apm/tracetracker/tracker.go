// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/tracetracker/tracker.go

package tracetracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/tracetracker"

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/log"
)

// fallbackEnvironment is the environment value to use if no environment is found in the span.
// This is the same value that is being set on the backend on spans that don't have an environment.
const fallbackEnvironment = "unknown"

// DefaultDimsToSyncSource are the default dimensions to sync correlated environment and services onto.
var DefaultDimsToSyncSource = map[string]string{
	"container_id":       "container_id",
	"kubernetes_pod_uid": "kubernetes_pod_uid",
}

// ActiveServiceTracker keeps track of which services are seen in the trace
// spans passed through ProcessSpans.  It supports expiry of service names if
// they are not seen for a certain amount of time.
type ActiveServiceTracker struct {
	log log.Logger

	// hostIDDims is the map of key/values discovered by the agent that identify the host
	hostIDDims map[string]string

	// hostServiceCache is a cache of services associated with the host
	hostServiceCache *TimeoutCache

	// hostEnvironmentCache is a cache of environments associated with the host
	hostEnvironmentCache *TimeoutCache

	// tenantServiceCache is the cache for services related to containers/pods
	tenantServiceCache *TimeoutCache

	// tenantEnvironmentCache is the cache for environments related to containers/pods
	tenantEnvironmentCache *TimeoutCache

	// tenantEnvironmentEmptyCache is the cache to make sure we don't send multiple requests to mitigate a very
	// specific corner case.  This is a separate cache so we don't impact metrics.  See the note in processEnvironment()
	// for more information
	tenantEmptyEnvironmentCache *TimeoutCache

	timeNow func() time.Time

	// correlationClient is the client used for updating infrastructure correlation properties
	correlationClient correlations.CorrelationClient

	// Map of dimensions to sync to with the key being the span attribute to lookup and the value being
	// the dimension to sync to.
	dimsToSyncSource map[string]string
}

// LoadHostIDDimCorrelations asynchronously retrieves all known correlations from the backend
// for all known hostIDDims.  This allows the agent to timeout and manage correlation
// deletions on restart.
func (a *ActiveServiceTracker) LoadHostIDDimCorrelations() {
	// asynchronously fetch all services and environments for each hostIDDim at startup
	for dimName, dimValue := range a.hostIDDims {
		a.correlationClient.Get(dimName, dimValue, func(correlations map[string][]string) {
			if services, ok := correlations["sf_services"]; ok {
				for _, service := range services {
					// Note that only the value is set for the host service cache because we only track services for the host
					// therefore there we don't need to include the dim key and value on the cache key
					if isNew := a.hostServiceCache.UpdateOrCreate(&CacheKey{value: service}, a.timeNow()); isNew {
						a.log.WithFields(log.Fields{"service": service}).Debug("Tracking service name from trace span")
					}
				}
			}
			if environments, ok := correlations["sf_environments"]; ok {
				// Note that only the value is set for the host environment cache because we only track environments for the host
				// therefore there we don't need to include the dim key and value on the cache key
				for _, environment := range environments {
					if isNew := a.hostEnvironmentCache.UpdateOrCreate(&CacheKey{value: environment}, a.timeNow()); isNew {
						a.log.WithFields(log.Fields{"environment": environment}).Debug("Tracking environment name from trace span")
					}
				}
			}
		})
	}
}

// New creates a new initialized service tracker
func New(
	log log.Logger,
	timeout time.Duration,
	correlationClient correlations.CorrelationClient,
	hostIDDims map[string]string,
	dimsToSyncSource map[string]string,
) *ActiveServiceTracker {
	a := &ActiveServiceTracker{
		log:                         log,
		hostIDDims:                  hostIDDims,
		hostServiceCache:            NewTimeoutCache(timeout),
		hostEnvironmentCache:        NewTimeoutCache(timeout),
		tenantServiceCache:          NewTimeoutCache(timeout),
		tenantEnvironmentCache:      NewTimeoutCache(timeout),
		tenantEmptyEnvironmentCache: NewTimeoutCache(timeout),
		correlationClient:           correlationClient,
		timeNow:                     time.Now,
		dimsToSyncSource:            dimsToSyncSource,
	}
	a.LoadHostIDDimCorrelations()

	return a
}

// ProcessTraces accepts a list of trace spans and uses them to update the
// current list of active services.  This is thread-safe.
func (a *ActiveServiceTracker) ProcessTraces(_ context.Context, traces ptrace.Traces) {
	// Take current time once since this is a system call.
	now := a.timeNow()

	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		a.processEnvironment(traces.ResourceSpans().At(i).Resource(), now)
		a.processService(traces.ResourceSpans().At(i).Resource(), now)
	}
}

func (a *ActiveServiceTracker) processEnvironment(res pcommon.Resource, now time.Time) {
	attrs := res.Attributes()
	if attrs.Len() == 0 {
		return
	}

	// Determine the environment value from the incoming spans.
	// First check "deployment.environment" attribute.
	// Then, try "environment" attribute (SignalFx schema).
	// Otherwise, use the same fallback value as set on the backend.
	var environment string
	if env, ok := attrs.Get(string(conventions.DeploymentEnvironmentKey)); ok {
		environment = env.Str()
	} else if env, ok = attrs.Get("environment"); ok {
		environment = env.Str()
	}
	if strings.TrimSpace(environment) == "" {
		environment = fallbackEnvironment
	}

	// update the environment for the hostIDDims
	// Note that only the value is set for the host environment cache because we only track environments for the host
	// therefore there we don't need to include the dim key and value on the cache key
	if exists := a.hostEnvironmentCache.UpdateIfExists(&CacheKey{value: environment}, now); !exists {
		if !a.hostEnvironmentCache.IsFull() {
			for dimName, dimValue := range a.hostIDDims {
				a.correlationClient.Correlate(&correlations.Correlation{
					Type:     correlations.Environment,
					DimName:  dimName,
					DimValue: dimValue,
					Value:    environment,
				}, func(_ *correlations.Correlation, err error) {
					if err == nil {
						a.hostEnvironmentCache.UpdateOrCreate(&CacheKey{value: environment}, now)
					}
					//nolint:errorlint
					if maxEntry, ok := err.(*correlations.ErrMaxEntries); ok && maxEntry.MaxEntries > 0 {
						a.hostEnvironmentCache.SetMaxSize(maxEntry.MaxEntries, now)
					}
				})
			}
		}
		a.log.WithFields(log.Fields{"environment": environment}).Debug("Tracking environment name from trace span")
	}

	// container / pod level stuff
	// this cache is necessary to identify environments associated with a kubernetes pod or container id
	for sourceAttr, dimName := range a.dimsToSyncSource {
		if val, ok := attrs.Get(sourceAttr); ok {
			// Note that the value is not set on the cache key.  We only send the first environment received for a
			// given pod/container, and we never delete the values set on the container/pod dimension.
			// So we only need to cache the dim name and dim value that have been associated with an environment.
			if exists := a.tenantEnvironmentCache.UpdateIfExists(&CacheKey{dimName: dimName, dimValue: val.Str()}, now); !exists {
				a.correlationClient.Correlate(&correlations.Correlation{
					Type:     correlations.Environment,
					DimName:  dimName,
					DimValue: val.Str(),
					Value:    environment,
				}, func(_ *correlations.Correlation, err error) {
					if err == nil {
						a.tenantEnvironmentCache.UpdateOrCreate(&CacheKey{dimName: dimName, dimValue: val.Str()}, now)
					}
				})
			}
		}
	}
}

func (a *ActiveServiceTracker) processService(res pcommon.Resource, now time.Time) {
	// Can't do anything if the spans don't have a local service name
	serviceNameAttr, ok := res.Attributes().Get(string(conventions.ServiceNameKey))
	service := serviceNameAttr.Str()
	if !ok || service == "" {
		return
	}

	// Handle host level service and environment correlation
	// Note that only the value is set for the host service cache because we only track services for the host
	// therefore there we don't need to include the dim key and value on the cache key
	if exists := a.hostServiceCache.UpdateIfExists(&CacheKey{value: service}, now); !exists {
		// all of the host id dims need to be correlated with the service
		for dimName, dimValue := range a.hostIDDims {
			if !a.hostServiceCache.IsFull() {
				a.correlationClient.Correlate(&correlations.Correlation{
					Type:     correlations.Service,
					DimName:  dimName,
					DimValue: dimValue,
					Value:    service,
				}, func(_ *correlations.Correlation, err error) {
					if err == nil {
						a.hostServiceCache.UpdateOrCreate(&CacheKey{value: service}, now)
					}
					//nolint:errorlint
					if maxEntry, ok := err.(*correlations.ErrMaxEntries); ok && maxEntry.MaxEntries > 0 {
						a.hostServiceCache.SetMaxSize(maxEntry.MaxEntries, now)
					}
				})
			}
		}

		a.log.WithFields(log.Fields{"service": service}).Debug("Tracking service name from trace span")
	}

	// container / pod level stuff (this should not directly affect the active service count)
	// this cache is necessary to identify services associated with a kubernetes pod or container id
	for sourceAttr, dimName := range a.dimsToSyncSource {
		if val, ok := res.Attributes().Get(sourceAttr); ok {
			// Note that the value is not set on the cache key.  We only send the first service received for a
			// given pod/container, and we never delete the values set on the container/pod dimension.
			// So we only need to cache the dim name and dim value that have been associated with a service.
			if exists := a.tenantServiceCache.UpdateIfExists(&CacheKey{dimName: dimName, dimValue: val.Str()}, now); !exists {
				a.correlationClient.Correlate(&correlations.Correlation{
					Type:     correlations.Service,
					DimName:  dimName,
					DimValue: val.Str(),
					Value:    service,
				}, func(_ *correlations.Correlation, err error) {
					if err == nil {
						a.tenantServiceCache.UpdateOrCreate(&CacheKey{dimName: dimName, dimValue: val.Str()}, now)
					}
				})
			}
		}
	}
}

// Purges caches on the ActiveServiceTracker
func (a *ActiveServiceTracker) Purge() {
	now := a.timeNow()

	for _, purged := range a.hostServiceCache.GetPurgeable(now) {
		// delete the correlation from all host id dims
		for dimName, dimValue := range a.hostIDDims {
			purged := purged
			a.correlationClient.Delete(&correlations.Correlation{
				Type:     correlations.Service,
				DimName:  dimName,
				DimValue: dimValue,
				Value:    purged.value,
			}, func(_ *correlations.Correlation) {
				a.hostServiceCache.Delete(purged)
			})
		}

		a.log.WithFields(log.Fields{"serviceName": purged.value}).Debug("No longer tracking service name from trace span")
	}

	for _, purged := range a.hostEnvironmentCache.GetPurgeable(now) {
		// delete the correlation from all host id dims
		for dimName, dimValue := range a.hostIDDims {
			purged := purged
			a.correlationClient.Delete(&correlations.Correlation{
				Type:     correlations.Environment,
				DimName:  dimName,
				DimValue: dimValue,
				Value:    purged.value,
			}, func(_ *correlations.Correlation) {
				a.hostEnvironmentCache.Delete(purged)
				a.log.WithFields(log.Fields{"environmentName": purged.value}).Debug("No longer tracking environment name from trace span")
			})
		}
	}

	// Purge the caches for containers and pods, but don't do the deletions.
	// These values aren't expected to change, and can be overwritten.
	// The onPurge() function doesn't need to do anything
	a.tenantServiceCache.PurgeOld(now, func(_ *CacheKey) {})
	a.tenantEnvironmentCache.PurgeOld(now, func(_ *CacheKey) {})
	a.tenantEmptyEnvironmentCache.PurgeOld(now, func(_ *CacheKey) {})
}
