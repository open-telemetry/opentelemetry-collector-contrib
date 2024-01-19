// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
)

// CollectorStatusDetails holds a snapshot of the current overall collector status, the overall
// pipeline statuses, and the statuses of the individual components within the pipelines.
type CollectorStatusDetails struct {
	OverallStatus      *component.StatusEvent
	PipelineStatusMap  map[component.ID]*component.StatusEvent
	ComponentStatusMap map[component.ID]map[*component.InstanceID]*component.StatusEvent
}

// PipelineStatusDetails holds a snapshot of the current overall pipeline status, and the statuses
// of the individual components in the pipeline.
type PipelineStatusDetails struct {
	OverallStatus      *component.StatusEvent
	ComponentStatusMap map[*component.InstanceID]*component.StatusEvent
}

type componentIDCache struct {
	mu             sync.RWMutex
	componentIDMap map[string]component.ID
}

func (c *componentIDCache) lookup(name string) (component.ID, error) {
	compID, ok := func() (component.ID, bool) {
		c.mu.RLock()
		defer c.mu.RUnlock()
		id, ok := c.componentIDMap[name]
		return id, ok
	}()

	if ok {
		return compID, nil
	}

	err := compID.UnmarshalText([]byte(name))
	if err == nil {
		c.mu.Lock()
		c.componentIDMap[name] = compID
		c.mu.Unlock()
	}

	return compID, err
}

// Extensions are treated as a pseudo pipeline and extsID is used as a map key
var extsID = component.NewID("extensions")
var extsIDMap = map[component.ID]struct{}{extsID: {}}

// The empty string is an alias for the overall collector health when subscribing to
// status events.
const emptyStream = ""

// CollectorID is used as a key in the subscriptions map
var collectorID = component.NewID("__collector__")

// Aggregator records individual status events for components and aggregates statuses for the
// pipelines they belong to and the collector overall.
type Aggregator struct {
	mu                 sync.RWMutex
	componentIDCache   *componentIDCache
	overallStatus      *component.StatusEvent
	pipelineStatusMap  map[component.ID]*component.StatusEvent
	componentStatusMap map[component.ID]map[*component.InstanceID]*component.StatusEvent
	subscriptions      map[component.ID][]chan *component.StatusEvent
}

// NewAggregator returns a *status.Aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{
		overallStatus:      &component.StatusEvent{},
		pipelineStatusMap:  make(map[component.ID]*component.StatusEvent),
		componentStatusMap: make(map[component.ID]map[*component.InstanceID]*component.StatusEvent),
		componentIDCache: &componentIDCache{
			componentIDMap: make(map[string]component.ID),
		},
		subscriptions: make(map[component.ID][]chan *component.StatusEvent),
	}
}

// CollectorStatus returns the overall status for the collector.
func (a *Aggregator) CollectorStatus() *component.StatusEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.overallStatus
}

// CollectorStatusDetailed returns a snapshot of the current overall collector status, pipeline
// statuses, and individual component statuses.
func (a *Aggregator) CollectorStatusDetailed() *CollectorStatusDetails {
	a.mu.RLock()
	defer a.mu.RUnlock()

	details := &CollectorStatusDetails{
		OverallStatus:     a.overallStatus,
		PipelineStatusMap: make(map[component.ID]*component.StatusEvent, len(a.pipelineStatusMap)),
		ComponentStatusMap: make(
			map[component.ID]map[*component.InstanceID]*component.StatusEvent,
			len(a.componentStatusMap),
		),
	}

	for compID, ev := range a.pipelineStatusMap {
		details.PipelineStatusMap[compID] = ev
	}

	for compID, eventMap := range a.componentStatusMap {
		details.ComponentStatusMap[compID] = make(
			map[*component.InstanceID]*component.StatusEvent,
			len(eventMap),
		)
		for instID, ev := range eventMap {
			details.ComponentStatusMap[compID][instID] = ev
		}
	}

	return details
}

// PipelineStatus returns the current overall pipeline status. An error will be returned if the
// pipeline is not found, or if there was an error marshaling the name to a component.ID.
func (a *Aggregator) PipelineStatus(name string) (*component.StatusEvent, error) {
	compID, err := a.componentIDCache.lookup(name)
	if err != nil {
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	ev, ok := a.pipelineStatusMap[compID]
	if !ok {
		return nil, fmt.Errorf("pipeline not found: %s", name)
	}

	return ev, nil
}

// PipelineStatusDetailed returns the current overall pipeline status and the invidiual statuses of
// the components within the pipeline. An error will be returned if the pipeline is not found, or if
// there was an error marshaling the name to a component.ID.
func (a *Aggregator) PipelineStatusDetailed(name string) (*PipelineStatusDetails, error) {
	compID, err := a.componentIDCache.lookup(name)
	if err != nil {
		return nil, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	ev, ok := a.pipelineStatusMap[compID]
	if !ok {
		return nil, fmt.Errorf("pipeline not found: %s", name)
	}

	details := &PipelineStatusDetails{
		OverallStatus: ev,
		ComponentStatusMap: make(
			map[*component.InstanceID]*component.StatusEvent,
			len(a.componentStatusMap),
		),
	}

	for instanceID, ev := range a.componentStatusMap[compID] {
		details.ComponentStatusMap[instanceID] = ev
	}

	return details, nil
}

// RecordStatus stores and aggregates a StatusEvent for the given component instance.
func (a *Aggregator) RecordStatus(source *component.InstanceID, event *component.StatusEvent) {
	compIDs := source.PipelineIDs
	// extensions are treated as a pseudo-pipeline
	if source.Kind == component.KindExtension {
		compIDs = extsIDMap
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for compID := range compIDs {
		var compStatuses map[*component.InstanceID]*component.StatusEvent
		compStatuses, ok := a.componentStatusMap[compID]
		if !ok {
			compStatuses = make(map[*component.InstanceID]*component.StatusEvent)
		}
		compStatuses[source] = event
		a.componentStatusMap[compID] = compStatuses

		pipelineStatus := component.AggregateStatusEvent(compStatuses)
		a.pipelineStatusMap[compID] = pipelineStatus
		a.notifySubscribers(compID, pipelineStatus)
	}

	overallStatus := component.AggregateStatusEvent(a.pipelineStatusMap)
	a.overallStatus = overallStatus
	a.notifySubscribers(collectorID, overallStatus)
}

// Subscribe allows you to subscribe to a stream of events for a pipline by passing in the
// pipeline name. The empty string can be used as an alias to subscribe to the collector health
// overall. It is possible to subscribe to a pipeline that has not yet reported. An initial nil
// will be sent on the channel and events will start streaming if and when it starts reporting.
func (a *Aggregator) Subscribe(name string) (<-chan *component.StatusEvent, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var compID component.ID
	var ev *component.StatusEvent

	if name == emptyStream {
		compID = collectorID
		ev = a.overallStatus
	} else {
		var err error
		compID, err = a.componentIDCache.lookup(name)
		if err != nil {
			return nil, err
		}
		ev = a.pipelineStatusMap[compID]
	}

	eventCh := make(chan *component.StatusEvent, 1)
	a.subscriptions[compID] = append(a.subscriptions[compID], eventCh)
	eventCh <- ev

	return eventCh, nil
}

// Unbsubscribe removes a stream from further status updates.
func (a *Aggregator) Unsubscribe(eventCh <-chan *component.StatusEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for compID, subs := range a.subscriptions {
		for i, sub := range subs {
			if sub == eventCh {
				a.subscriptions[compID] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

// Close terminates all existing subscriptions.
func (a *Aggregator) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, subs := range a.subscriptions {
		for _, sub := range subs {
			close(sub)
		}
	}
}

func (a *Aggregator) notifySubscribers(compID component.ID, event *component.StatusEvent) {
	for _, sub := range a.subscriptions[compID] {
		// clear unread events
		select {
		case <-sub:
		default:
		}
		sub <- event
	}
}
