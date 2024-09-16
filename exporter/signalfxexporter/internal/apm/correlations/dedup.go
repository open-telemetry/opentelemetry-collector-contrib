// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/correlations/dedup.go

package correlations // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"

import (
	"container/list"
	"net/http"
)

// deduplicator deduplicates requests and cancels pending conflicting requests and deduplicates
// this is not threadsafe
type deduplicator struct {
	// maps for deduplicating requests
	maxSize           int
	pendingCreates    *list.List
	pendingCreateKeys map[Correlation]*list.Element
	pendingDeletes    *list.List
	pendingDeleteKeys map[Correlation]*list.Element
}

func (d *deduplicator) purgeCreates() {
	var elem = d.pendingCreates.Front()
	for {
		if elem == nil {
			return
		}
		if elem.Value.(*request).ctx.Err() != nil {
			toDelete := elem
			elem = elem.Next()
			d.pendingCreates.Remove(toDelete)
			delete(d.pendingCreateKeys, *toDelete.Value.(*request).Correlation)
		} else {
			elem = elem.Next()
		}
	}
}

func (d *deduplicator) purgeDeletes() {
	var elem = d.pendingDeletes.Front()
	for {
		if elem == nil {
			return
		}
		if elem.Value.(*request).ctx.Err() != nil {
			toDelete := elem
			elem = elem.Next()
			d.pendingDeletes.Remove(toDelete)
			delete(d.pendingDeleteKeys, *toDelete.Value.(*request).Correlation)
		} else {
			elem = elem.Next()
		}
	}
}

func (d *deduplicator) purge() {
	d.purgeCreates()
	d.purgeDeletes()
}

func (d *deduplicator) evictPendingDelete() {
	var elem = d.pendingDeletes.Back()
	if elem != nil {
		req, ok := elem.Value.(*request)
		if ok {
			req.cancel()
			d.pendingDeletes.Remove(elem)
			delete(d.pendingDeleteKeys, *req.Correlation)
		}
	}
}

func (d *deduplicator) evictPendingCreate() {
	var elem = d.pendingCreates.Back()
	if elem != nil {
		req, ok := elem.Value.(*request)
		if ok {
			req.cancel()
			d.pendingCreates.Remove(elem)
			delete(d.pendingCreateKeys, *req.Correlation)

		}
	}
}

func (d *deduplicator) dedupCorrelate(r *request) bool {
	// look for duplicate pending creates
	pendingCreate, ok := d.pendingCreateKeys[*r.Correlation]
	if ok && pendingCreate.Value.(*request).ctx.Err() == nil {
		// return true if there is a context for the key and the context has not expired
		return true
	}

	// make room if necessary
	if len(d.pendingCreateKeys) >= d.maxSize {
		d.evictPendingCreate()
	}

	// insert the request into the pendingCreates
	elem := d.pendingCreates.PushFront(r)
	d.pendingCreateKeys[*r.Correlation] = elem

	// cancel any pending delete operations
	deleteElem, pendindgDelete := d.pendingDeleteKeys[*r.Correlation]
	if pendindgDelete {
		deleteElem.Value.(*request).cancel()
		d.pendingDeletes.Remove(deleteElem)
		delete(d.pendingDeleteKeys, *deleteElem.Value.(*request).Correlation)
	}

	return false
}

func (d *deduplicator) dedupDelete(r *request) bool {
	// look for duplicate pending creates
	pendingDelete, ok := d.pendingDeleteKeys[*r.Correlation]
	if ok && pendingDelete.Value.(*request).ctx.Err() == nil {
		// return true if there is a context for the key and the context has not expired
		return true
	}

	// make room if necessary
	if len(d.pendingDeleteKeys) >= d.maxSize {
		d.evictPendingDelete()
	}

	// insert the request into the pendingDeletes
	elem := d.pendingDeletes.PushFront(r)
	d.pendingDeleteKeys[*r.Correlation] = elem

	// cancel any pending create operations
	createElem, pendindgCreate := d.pendingCreateKeys[*r.Correlation]
	if pendindgCreate {
		createElem.Value.(*request).cancel()
		d.pendingCreates.Remove(createElem)
		delete(d.pendingCreateKeys, *createElem.Value.(*request).Correlation)
	}

	return false
}

// isDup returns true if the request is a duplicate
func (d *deduplicator) isDup(r *request) (isDup bool) {
	switch r.operation {
	case http.MethodPut:
		return d.dedupCorrelate(r)
	case http.MethodDelete:
		return d.dedupDelete(r)
	default:
		return
	}
}

// newDeduplicator returns a new instance
func newDeduplicator(size int) *deduplicator {
	return &deduplicator{
		maxSize:           size,
		pendingCreates:    list.New(),
		pendingCreateKeys: make(map[Correlation]*list.Element),
		pendingDeletes:    list.New(),
		pendingDeleteKeys: make(map[Correlation]*list.Element),
	}
}
