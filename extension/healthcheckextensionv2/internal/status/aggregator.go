// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"

import (
	"fmt"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
)

// Extensions are treated as a pseudo pipeline and extsID is used as a map key
var (
	extsID    = component.NewID("extensions")
	extsIDMap = map[component.ID]struct{}{extsID: {}}
)

// Scope refers to a part of an AggregateStatus. The zero-value, aka ScopeAll,
// refers to the entire AggregateStatus. ScopeExtensions refers to the extensions
// subtree, and any other value refers to a pipeline subtree.
type Scope string

const (
	ScopeAll        Scope  = ""
	ScopeExtensions Scope  = "extensions"
	pipelinePrefix  string = "pipeline:"
)

func (s Scope) toKey() string {
	if s == ScopeAll || s == ScopeExtensions {
		return string(s)
	}
	return pipelinePrefix + string(s)
}

// Detail specifies whether or not to include subtrees with an AggregateStatus
type Detail bool

const (
	IncludeSubtrees Detail = true
	ExcludeSubtrees Detail = false
)

// AggregateStatus contains a map of child AggregateStatuses and an embedded component.StatusEvent.
// It can be used to represent a single, top-level status when the ComponentStatusMap is empty,
// or a nested structure when map is non-empty.
type AggregateStatus struct {
	*component.StatusEvent

	ComponentStatusMap map[string]*AggregateStatus
}

func (a *AggregateStatus) clone(detail Detail) *AggregateStatus {
	st := &AggregateStatus{
		StatusEvent: a.StatusEvent,
	}

	if detail == IncludeSubtrees && len(a.ComponentStatusMap) > 0 {
		st.ComponentStatusMap = make(map[string]*AggregateStatus, len(a.ComponentStatusMap))
		for k, cs := range a.ComponentStatusMap {
			st.ComponentStatusMap[k] = cs.clone(detail)
		}
	}

	return st
}

type subscription struct {
	statusCh chan *AggregateStatus
	detail   Detail
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
// overall (ScopeAll), extensions (ScopeExtensions), or a pipeline by name. Detail specifies whether
// or not subtrees should be returned with the *AggregateStatus. The boolean return value indicates
// whether or not the scope was found.
func (a *Aggregator) AggregateStatus(scope Scope, detail Detail) (*AggregateStatus, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if scope == ScopeAll {
		return a.aggregateStatus.clone(detail), true
	}

	st, ok := a.aggregateStatus.ComponentStatusMap[scope.toKey()]
	if !ok {
		return nil, false
	}

	return st.clone(detail), true
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
		var pipelineStatus *AggregateStatus
		pipelineScope := Scope(compID.String())
		pipelineKey := pipelineScope.toKey()

		pipelineStatus, ok := a.aggregateStatus.ComponentStatusMap[pipelineKey]
		if !ok {
			pipelineStatus = &AggregateStatus{
				ComponentStatusMap: make(map[string]*AggregateStatus),
			}
		}

		componentKey := fmt.Sprintf("%s:%s", strings.ToLower(source.Kind.String()), source.ID)
		pipelineStatus.ComponentStatusMap[componentKey] = &AggregateStatus{
			StatusEvent: event,
		}
		a.aggregateStatus.ComponentStatusMap[pipelineKey] = pipelineStatus
		pipelineStatus.StatusEvent = component.AggregateStatusEvent(
			toStatusEventMap(pipelineStatus),
		)
		a.notifySubscribers(pipelineScope, pipelineStatus)
	}

	a.aggregateStatus.StatusEvent = component.AggregateStatusEvent(
		toStatusEventMap(a.aggregateStatus),
	)
	a.notifySubscribers(ScopeAll, a.aggregateStatus)
}

// Subscribe allows you to subscribe to a stream of events for the given scope. The scope can be
// the collector overall (ScopeAll), extensions (ScopeExtensions), or a pipeline name.
// It is possible to subscribe to a pipeline that has not yet reported. An initial nil
// will be sent on the channel and events will start streaming if and when it starts reporting.
// Detail specifies whether or not subtrees should be returned with the *AggregateStatus.
func (a *Aggregator) Subscribe(scope Scope, detail Detail) <-chan *AggregateStatus {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := scope.toKey()
	st := a.aggregateStatus
	if scope != ScopeAll {
		st = st.ComponentStatusMap[key]
	}

	sub := &subscription{
		statusCh: make(chan *AggregateStatus, 1),
		detail:   detail,
	}

	a.subscriptions[key] = append(a.subscriptions[key], sub)
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

func (a *Aggregator) notifySubscribers(scope Scope, status *AggregateStatus) {
	for _, sub := range a.subscriptions[scope.toKey()] {
		// clear unread events
		select {
		case <-sub.statusCh:
		default:
		}
		sub.statusCh <- status.clone(sub.detail)
	}
}

func toStatusEventMap(aggStatus *AggregateStatus) map[string]*component.StatusEvent {
	result := make(map[string]*component.StatusEvent, len(aggStatus.ComponentStatusMap))
	for k, v := range aggStatus.ComponentStatusMap {
		result[k] = v.StatusEvent
	}
	return result
}
