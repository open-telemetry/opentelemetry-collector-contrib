// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package status // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"

import (
	"container/list"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

// Note: this interface had to be introduced because we need to be able to rewrite the
// timestamps of some events during aggregation. The implementation in core doesn't currently
// allow this, but this interface provides a workaround.
type Event interface {
	Status() componentstatus.Status
	Err() error
	Timestamp() time.Time
}

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
	switch s {
	case ScopeAll, ScopeExtensions:
		return string(s)
	default:
		return pipelinePrefix + string(s)
	}
}

type Verbosity bool

const (
	Verbose Verbosity = true
	Concise           = false
)

// AggregateStatus contains a map of child AggregateStatuses and an embedded Event.
// It can be used to represent a single, top-level status when the ComponentStatusMap
// is empty, or a nested structure when map is non-empty.
type AggregateStatus struct {
	Event

	ComponentStatusMap map[string]*AggregateStatus
}

func (a *AggregateStatus) clone(verbosity Verbosity) *AggregateStatus {
	st := &AggregateStatus{
		Event: a.Event,
	}

	if verbosity == Verbose && len(a.ComponentStatusMap) > 0 {
		st.ComponentStatusMap = make(map[string]*AggregateStatus, len(a.ComponentStatusMap))
		for k, cs := range a.ComponentStatusMap {
			st.ComponentStatusMap[k] = cs.clone(verbosity)
		}
	}

	return st
}

type subscription struct {
	statusCh  chan *AggregateStatus
	verbosity Verbosity
}

// UnsubscribeFunc is a function used to unsubscribe from a stream.
type UnsubscribeFunc func()

// Aggregator records individual status events for components and aggregates statuses for the
// pipelines they belong to and the collector overall.
type Aggregator struct {
	// mu protects aggregateStatus and subscriptions from concurrent modification
	mu              sync.RWMutex
	aggregateStatus *AggregateStatus
	subscriptions   map[string]*list.List
	aggregationFunc aggregationFunc
}

// NewAggregator returns a *status.Aggregator.
func NewAggregator(errPriority ErrorPriority) *Aggregator {
	return &Aggregator{
		aggregateStatus: &AggregateStatus{
			Event:              &componentstatus.Event{},
			ComponentStatusMap: make(map[string]*AggregateStatus),
		},
		subscriptions:   make(map[string]*list.List),
		aggregationFunc: newAggregationFunc(errPriority),
	}
}

// AggregateStatus returns an *AggregateStatus for the given scope. The scope can be the collector
// overall (ScopeAll), extensions (ScopeExtensions), or a pipeline by name. Detail specifies whether
// or not subtrees should be returned with the *AggregateStatus. The boolean return value indicates
// whether or not the scope was found.
func (a *Aggregator) AggregateStatus(scope Scope, verbosity Verbosity) (*AggregateStatus, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if scope == ScopeAll {
		return a.aggregateStatus.clone(verbosity), true
	}

	st, ok := a.aggregateStatus.ComponentStatusMap[scope.toKey()]
	if !ok {
		return nil, false
	}

	return st.clone(verbosity), true
}

// RecordStatus stores and aggregates a StatusEvent for the given component instance.
func (a *Aggregator) RecordStatus(source *componentstatus.InstanceID, event *componentstatus.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// extensions are treated as a pseudo-pipeline
	if source.Kind() == component.KindExtension {
		a.updateStatus(ScopeExtensions, source, event)
	} else {
		source.AllPipelineIDs(func(id pipeline.ID) bool {
			a.updateStatus(Scope(id.String()), source, event)
			return true
		})
	}

	a.aggregateStatus.Event = a.aggregationFunc(a.aggregateStatus)
	a.notifySubscribers(ScopeAll, a.aggregateStatus)
}

func (a *Aggregator) updateStatus(pipelineScope Scope, source *componentstatus.InstanceID, event *componentstatus.Event) {
	pipelineKey := pipelineScope.toKey()
	pipelineStatus, ok := a.aggregateStatus.ComponentStatusMap[pipelineKey]
	if !ok {
		pipelineStatus = &AggregateStatus{
			ComponentStatusMap: make(map[string]*AggregateStatus),
		}
		a.aggregateStatus.ComponentStatusMap[pipelineKey] = pipelineStatus
	}

	componentKey := strings.ToLower(source.Kind().String()) + ":" + source.ComponentID().String()
	pipelineStatus.ComponentStatusMap[componentKey] = &AggregateStatus{
		Event: event,
	}
	pipelineStatus.Event = a.aggregationFunc(pipelineStatus)
	a.notifySubscribers(pipelineScope, pipelineStatus)
}

// Subscribe allows you to subscribe to a stream of events for the given scope. The scope can be
// the collector overall (ScopeAll), extensions (ScopeExtensions), or a pipeline name.
// It is possible to subscribe to a pipeline that has not yet reported. An initial nil
// will be sent on the channel and events will start streaming if and when it starts reporting.
// A `Verbose` verbosity specifies that subtrees should be returned with the *AggregateStatus.
// To unsubscribe, call the returned UnsubscribeFunc.
func (a *Aggregator) Subscribe(scope Scope, verbosity Verbosity) (<-chan *AggregateStatus, UnsubscribeFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := scope.toKey()
	st := a.aggregateStatus
	if scope != ScopeAll {
		st = st.ComponentStatusMap[key]
	}
	if st != nil {
		st = st.clone(verbosity)
	}
	sub := &subscription{
		statusCh:  make(chan *AggregateStatus, 1),
		verbosity: verbosity,
	}
	subList, ok := a.subscriptions[key]
	if !ok {
		subList = list.New()
		a.subscriptions[key] = subList
	}
	el := subList.PushBack(sub)

	unsubFunc := func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		subList.Remove(el)
		if subList.Front() == nil {
			delete(a.subscriptions, key)
		}
	}

	sub.statusCh <- st

	return sub.statusCh, unsubFunc
}

// Close terminates all existing subscriptions.
func (a *Aggregator) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, subList := range a.subscriptions {
		for el := subList.Front(); el != nil; el = el.Next() {
			sub := el.Value.(*subscription)
			close(sub.statusCh)
		}
	}
}

func (a *Aggregator) notifySubscribers(scope Scope, status *AggregateStatus) {
	subList, ok := a.subscriptions[scope.toKey()]
	if !ok {
		return
	}
	for el := subList.Front(); el != nil; el = el.Next() {
		sub := el.Value.(*subscription)
		// clear unread events
		select {
		case <-sub.statusCh:
		default:
		}
		sub.statusCh <- status.clone(sub.verbosity)
	}
}
