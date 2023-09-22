// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/tracetracker/tracker.go

package tracetracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/tracetracker"

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/log"
)

// DefaultDimsToSyncSource are the default dimensions to sync correlated environment and services onto.
var DefaultDimsToSyncSource = map[string]string{
	"container_id":       "container_id",
	"kubernetes_pod_uid": "kubernetes_pod_uid",
}

// ActiveServiceTracker keeps track of which services are seen in the trace
// spans passed through ProcessSpans.  It supports expiry of service names if
// they are not seen for a certain amount of time.
type ActiveServiceTracker struct {
	dpCacheLock sync.Mutex

	log log.Logger

	// hostIDDims is the map of key/values discovered by the agent that identify the host
	hostIDDims map[string]string

	// sendTraceHostCorrelationMetrics turns metric emission on and off
	sendTraceHostCorrelationMetrics bool

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

	// cache of service names to generate datapoints for
	dpCache map[string]struct{}

	timeNow func() time.Time

	// correlationClient is the client used for updating infrastructure correlation properties
	correlationClient correlations.CorrelationClient

	// Internal metrics
	spansProcessed int64

	// Map of dimensions to sync to with the key being the span attribute to lookup and the value being
	// the dimension to sync to.
	dimsToSyncSource map[string]string
}

// addServiceToDPCache creates a datapoint for the given service in the dpCache.
func (a *ActiveServiceTracker) addServiceToDPCache(service string) {
	a.dpCacheLock.Lock()
	defer a.dpCacheLock.Unlock()

	a.dpCache[service] = struct{}{}
}

// removeServiceFromDPCache removes the datapoint for the given service from the dpCache
func (a *ActiveServiceTracker) removeServiceFromDPCache(service string) {
	a.dpCacheLock.Lock()
	delete(a.dpCache, service)
	a.dpCacheLock.Unlock()
}

// LoadHostIDDimCorrelations asynchronously retrieves all known correlations from the backend
// for all known hostIDDims.  This allows the agent to timeout and manage correlation
// deletions on restart.
func (a *ActiveServiceTracker) LoadHostIDDimCorrelations() {
	// asynchronously fetch all services and environments for each hostIDDim at startup
	for dimName, dimValue := range a.hostIDDims {
		dimName := dimName
		dimValue := dimValue
		a.correlationClient.Get(dimName, dimValue, func(correlations map[string][]string) {
			if services, ok := correlations["sf_services"]; ok {
				for _, service := range services {
					// Note that only the value is set for the host service cache because we only track services for the host
					// therefore there we don't need to include the dim key and value on the cache key
					if isNew := a.hostServiceCache.UpdateOrCreate(&CacheKey{value: service}, a.timeNow()); isNew {
						if a.sendTraceHostCorrelationMetrics {
							// create datapoint for service
							a.addServiceToDPCache(service)
						}

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
	sendTraceHostCorrelationMetrics bool,
	dimsToSyncSource map[string]string,
) *ActiveServiceTracker {
	a := &ActiveServiceTracker{
		log:                             log,
		hostIDDims:                      hostIDDims,
		hostServiceCache:                NewTimeoutCache(timeout),
		hostEnvironmentCache:            NewTimeoutCache(timeout),
		tenantServiceCache:              NewTimeoutCache(timeout),
		tenantEnvironmentCache:          NewTimeoutCache(timeout),
		tenantEmptyEnvironmentCache:     NewTimeoutCache(timeout),
		dpCache:                         make(map[string]struct{}),
		correlationClient:               correlationClient,
		sendTraceHostCorrelationMetrics: sendTraceHostCorrelationMetrics,
		timeNow:                         time.Now,
		dimsToSyncSource:                dimsToSyncSource,
	}
	a.LoadHostIDDimCorrelations()

	return a
}

// AddSpansGeneric accepts a list of trace spans and uses them to update the
// current list of active services.  This is thread-safe.
func (a *ActiveServiceTracker) AddSpansGeneric(_ context.Context, spans SpanList) {
	// Take current time once since this is a system call.
	now := a.timeNow()

	for i := 0; i < spans.Len(); i++ {
		a.processEnvironment(spans.At(i), now)
		a.processService(spans.At(i), now)
	}

	// Protected by lock above
	atomic.AddInt64(&a.spansProcessed, int64(spans.Len()))
}

func (a *ActiveServiceTracker) processEnvironment(span Span, now time.Time) {
	if span.NumTags() == 0 {
		return
	}
	environment, environmentFound := span.Environment()

	if !environmentFound || strings.TrimSpace(environment) == "" {
		// The following is ONLY to mitigate a corner case scenario where the environment for a container/pod is set on
		// the backend with an old default environment set by the agent, and the agent has been restarted with no
		// default environment. On restart, the agent only fetches existing environment values for hostIDDims, and does
		// not fetch for containers/pod dims. If a container/pod is emitting spans without an environment value, then
		// the agent won't be able to overwrite the value. The agent is also unable to age out environment values for
		// containers/pods from startup.
		//
		// Under that VERY specific circumstance, we need to fetch and delete the environment values for each
		// pod/container that we have not already scraped an environment off of this agent runtime.
		for sourceAttr, dimName := range a.dimsToSyncSource {
			sourceAttr := sourceAttr
			dimName := dimName
			if dimValue, ok := span.Tag(sourceAttr); ok {
				// look up the dimension / value in the environment cache to ensure it doesn't already exist
				// if it does exist, this means we've already scraped and overwritten what was on the backend
				// probably from another span. This also implies that some spans for the tenant have an environment
				// and some do not.
				a.tenantEnvironmentCache.RunIfKeyDoesNotExist(&CacheKey{dimName: dimName, dimValue: dimValue}, func() {
					// create a cache key ensuring that we don't fetch environment values multiple times for spans with
					// empty environments
					if isNew := a.tenantEmptyEnvironmentCache.UpdateOrCreate(&CacheKey{dimName: dimName, dimValue: dimValue}, now); isNew {
						// get the existing value from the backend
						a.correlationClient.Get(dimName, dimValue, func(response map[string][]string) {
							if len(response) == 0 {
								return
							}

							// look for the existing environment value
							environments, ok := response["sf_environments"]
							if !ok || len(environments) == 0 {
								return
							}

							// Note: This cache operation is OK to execute inside of the encapsulating
							// tenantEnvironmentCache.RunIfKeyDoesNotExist() because it is actually inside an
							// asynchronous callback to the correlation client's Get(). So... by the time the callback
							// is actually executed, the parent RunIfKeyDoesNotExist will have already released the lock
							// on the cache
							a.tenantEnvironmentCache.RunIfKeyDoesNotExist(&CacheKey{dimName: dimName, dimValue: dimValue}, func() {
								a.correlationClient.Delete(&correlations.Correlation{
									Type:     correlations.Environment,
									DimName:  dimName,
									DimValue: dimValue,
									Value:    environments[0], // We already checked for empty, and backend enforces 1 value max.
								}, func(_ *correlations.Correlation) {})
							})
						})
					}
				})
			}
		}

		// return so we don't set empty string or spaces as an environment value
		return
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
				}, func(cor *correlations.Correlation, err error) {
					if err == nil {
						a.hostEnvironmentCache.UpdateOrCreate(&CacheKey{value: environment}, now)
					}
					// nolint:errorlint
					if max, ok := err.(*correlations.ErrMaxEntries); ok && max.MaxEntries > 0 {
						a.hostEnvironmentCache.SetMaxSize(max.MaxEntries, now)
					}
				})
			}
		}
		a.log.WithFields(log.Fields{"environment": environment}).Debug("Tracking environment name from trace span")
	}

	// container / pod level stuff
	// this cache is necessary to identify environments associated with a kubernetes pod or container id
	for sourceAttr, dimName := range a.dimsToSyncSource {
		sourceAttr := sourceAttr
		dimName := dimName
		if dimValue, ok := span.Tag(sourceAttr); ok {
			// Note that the value is not set on the cache key.  We only send the first environment received for a
			// given pod/container, and we never delete the values set on the container/pod dimension.
			// So we only need to cache the dim name and dim value that have been associated with an environment.
			if exists := a.tenantEnvironmentCache.UpdateIfExists(&CacheKey{dimName: dimName, dimValue: dimValue}, now); !exists {
				a.correlationClient.Correlate(&correlations.Correlation{
					Type:     correlations.Environment,
					DimName:  dimName,
					DimValue: dimValue,
					Value:    environment,
				}, func(cor *correlations.Correlation, err error) {
					if err == nil {
						a.tenantEnvironmentCache.UpdateOrCreate(&CacheKey{dimName: dimName, dimValue: dimValue}, now)
					}
				})
			}
		}
	}
}

func (a *ActiveServiceTracker) processService(span Span, now time.Time) {
	// Can't do anything if the spans don't have a local service name
	service, ok := span.ServiceName()
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
				}, func(cor *correlations.Correlation, err error) {
					if err == nil {
						a.hostServiceCache.UpdateOrCreate(&CacheKey{value: service}, now)
					}
					// nolint:errorlint
					if max, ok := err.(*correlations.ErrMaxEntries); ok && max.MaxEntries > 0 {
						a.hostServiceCache.SetMaxSize(max.MaxEntries, now)
					}
				})
			}
		}

		if a.sendTraceHostCorrelationMetrics {
			// create datapoint for service
			a.addServiceToDPCache(service)
		}

		a.log.WithFields(log.Fields{"service": service}).Debug("Tracking service name from trace span")
	}

	// container / pod level stuff (this should not directly affect the active service count)
	// this cache is necessary to identify services associated with a kubernetes pod or container id
	for sourceAttr, dimName := range a.dimsToSyncSource {
		sourceAttr := sourceAttr
		dimName := dimName
		if dimValue, ok := span.Tag(sourceAttr); ok {
			// Note that the value is not set on the cache key.  We only send the first service received for a
			// given pod/container, and we never delete the values set on the container/pod dimension.
			// So we only need to cache the dim name and dim value that have been associated with a service.
			if exists := a.tenantServiceCache.UpdateIfExists(&CacheKey{dimName: dimName, dimValue: dimValue}, now); !exists {
				a.correlationClient.Correlate(&correlations.Correlation{
					Type:     correlations.Service,
					DimName:  dimName,
					DimValue: dimValue,
					Value:    service,
				}, func(cor *correlations.Correlation, err error) {
					if err == nil {
						a.tenantServiceCache.UpdateOrCreate(&CacheKey{dimName: dimName, dimValue: dimValue}, now)
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
			}, func(cor *correlations.Correlation) {
				a.hostServiceCache.Delete(purged)
			})
		}
		// remove host/service correlation metric from tracker
		if a.sendTraceHostCorrelationMetrics {
			a.removeServiceFromDPCache(purged.value)
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
			}, func(cor *correlations.Correlation) {
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
