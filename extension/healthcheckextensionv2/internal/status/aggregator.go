// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
)

// Extensions are treated as a pseudo pipeline and extsID is used as a map key
var (
	extsID    = component.NewID("extensions")
	extsIDMap = map[component.ID]struct{}{extsID: {}}
)

const (
	ScopeAll        = ""
	ScopeExtensions = "extensions"
	pipelinePrefix  = "pipeline:"
)

// AggregateStatus contains a map of child AggregateStatuses and an embedded component.StatusEvent.
// It can be used to represent a single, top-level status when the ComponentStatusMap is empty,
// or a nested structure when map is non-empty.
type AggregateStatus struct {
	*component.StatusEvent

	ComponentStatusMap map[string]*AggregateStatus
}

func (a *AggregateStatus) clone(detailed bool) *AggregateStatus {
	st := &AggregateStatus{
		StatusEvent: a.StatusEvent,
	}

	if detailed && len(a.ComponentStatusMap) > 0 {
		st.ComponentStatusMap = make(map[string]*AggregateStatus, len(a.ComponentStatusMap))
		for k, cs := range a.ComponentStatusMap {
			st.ComponentStatusMap[k] = cs.clone(detailed)
		}
	}

	return st
}

type subscription struct {
	statusCh chan *AggregateStatus
	detailed bool
}

// Aggregator records individual status events for components and aggregates statuses for the
// pipelines they belong to and the collector overall.
type Aggregator struct {
	mu              sync.RWMutex
	aggregateStatus *AggregateStatus
	subscriptions   map[string][]*subscription
}

// NewAggregator returns a *status.Aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{
		aggregateStatus: &AggregateStatus{
			StatusEvent:        &component.StatusEvent{},
			ComponentStatusMap: make(map[string]*AggregateStatus),
		},
		subscriptions: make(map[string][]*subscription),
	}
}

// AggregateStatus returns an *AggregateStatus for the given scope. The scope can be the collector
// overall (represented the empty string), extensions, or a pipeline by name. If detailed is true,
// the *AggregateStatus will contain child component statuses for the given scope. If it's false,
// only a top level status will be returned. The boolean return value indicates whether or not the
// scope was found.
func (a *Aggregator) AggregateStatus(scope string, detailed bool) (*AggregateStatus, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if scope == ScopeAll {
		return a.aggregateStatus.clone(detailed), true
	}

	if scope != ScopeExtensions {
		scope = pipelinePrefix + scope
	}

	st, ok := a.aggregateStatus.ComponentStatusMap[scope]
	if !ok {
		return nil, false
	}

	return st.clone(detailed), true
}

// RecordStatus stores and aggregates a StatusEvent for the given component instance.
func (a *Aggregator) RecordStatus(source *component.InstanceID, event *component.StatusEvent) {
	compIDs := source.PipelineIDs
	prefix := pipelinePrefix
	// extensions are treated as a pseudo-pipeline
	if source.Kind == component.KindExtension {
		compIDs = extsIDMap
		prefix = ""
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for compID := range compIDs {
		var pipelineStatus *AggregateStatus
		pipelineKey := prefix + compID.String()
		pipelineStatus, ok := a.aggregateStatus.ComponentStatusMap[pipelineKey]
		if !ok {
			pipelineStatus = &AggregateStatus{
				ComponentStatusMap: make(map[string]*AggregateStatus),
			}
		}

		componentKey := fmt.Sprintf("%s:%s", kindToString(source.Kind), source.ID.String())
		pipelineStatus.ComponentStatusMap[componentKey] = &AggregateStatus{
			StatusEvent: event,
		}
		a.aggregateStatus.ComponentStatusMap[pipelineKey] = pipelineStatus
		pipelineStatus.StatusEvent = component.AggregateStatusEvent(
			toStatusEventMap(pipelineStatus),
		)
		a.notifySubscribers(pipelineKey, pipelineStatus)
	}

	a.aggregateStatus.StatusEvent = component.AggregateStatusEvent(
		toStatusEventMap(a.aggregateStatus),
	)
	a.notifySubscribers(ScopeAll, a.aggregateStatus)
}

// Subscribe allows you to subscribe to a stream of events for the given scope. The scope can be
// the collector overall (represented by the emptry string), extensions, or a pipeline name.
// It is possible to subscribe to a pipeline that has not yet reported. An initial nil
// will be sent on the channel and events will start streaming if and when it starts reporting. If
// detailed is true, the *AggregateStatus will contain child statuses for the given scope. If it's
// false it will contain only a top-level status.
func (a *Aggregator) Subscribe(scope string, detailed bool) <-chan *AggregateStatus {
	a.mu.Lock()
	defer a.mu.Unlock()

	st := a.aggregateStatus
	if scope != ScopeAll {
		if scope != ScopeExtensions {
			scope = pipelinePrefix + scope
		}
		st = st.ComponentStatusMap[scope]
	}

	sub := &subscription{
		statusCh: make(chan *AggregateStatus, 1),
		detailed: detailed,
	}

	a.subscriptions[scope] = append(a.subscriptions[scope], sub)
	sub.statusCh <- st

	return sub.statusCh
}

// Unbsubscribe removes a stream from further status updates.
func (a *Aggregator) Unsubscribe(statusCh <-chan *AggregateStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for scope, subs := range a.subscriptions {
		for i, sub := range subs {
			if sub.statusCh == statusCh {
				a.subscriptions[scope] = append(subs[:i], subs[i+1:]...)
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
			close(sub.statusCh)
		}
	}
}

func (a *Aggregator) notifySubscribers(scope string, status *AggregateStatus) {
	for _, sub := range a.subscriptions[scope] {
		// clear unread events
		select {
		case <-sub.statusCh:
		default:
		}
		sub.statusCh <- status.clone(sub.detailed)
	}
}

func toStatusEventMap(aggStatus *AggregateStatus) map[string]*component.StatusEvent {
	result := make(map[string]*component.StatusEvent, len(aggStatus.ComponentStatusMap))
	for k, v := range aggStatus.ComponentStatusMap {
		result[k] = v.StatusEvent
	}
	return result
}

func kindToString(k component.Kind) string {
	switch k {
	case component.KindReceiver:
		return "receiver"
	case component.KindProcessor:
		return "processor"
	case component.KindExporter:
		return "exporter"
	case component.KindExtension:
		return "extension"
	case component.KindConnector:
		return "connector"
	}
	return ""
}
