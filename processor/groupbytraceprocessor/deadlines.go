// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"container/heap"
	"time"
)

// subtraceDeadlines orders subtraces by when they next fall due for release. It
// holds at most one deadline per subtrace.
//
// A worker owns its deadlines: a deadline's lifecycle matches the ring buffer
// entry it belongs to, so there is never a deadline for a subtrace the worker
// has stopped tracking.
type subtraceDeadlines struct {
	pq    deadlinePQ
	index map[subtraceID]*deadlineEntry
}

type deadlineEntry struct {
	id  subtraceID
	due time.Time

	// at is the entry's position in pq, maintained by the heap so that an entry
	// found through index can be reordered or removed without searching for it.
	at int
}

func newSubtraceDeadlines() *subtraceDeadlines {
	return &subtraceDeadlines{index: make(map[subtraceID]*deadlineEntry)}
}

// set gives the subtrace a deadline, replacing whatever it had.
func (d *subtraceDeadlines) set(id subtraceID, due time.Time) {
	if e, held := d.index[id]; held {
		e.due = due
		heap.Fix(&d.pq, e.at)
		return
	}

	e := &deadlineEntry{id: id, due: due}
	d.index[id] = e
	heap.Push(&d.pq, e)
}

// remove drops the subtrace's deadline, if it has one.
func (d *subtraceDeadlines) remove(id subtraceID) {
	e, held := d.index[id]
	if !held {
		return
	}

	heap.Remove(&d.pq, e.at)
	delete(d.index, id)
}

// next reports when the earliest deadline falls due, and whether there is one.
func (d *subtraceDeadlines) next() (time.Time, bool) {
	if len(d.pq) == 0 {
		return time.Time{}, false
	}
	return d.pq[0].due, true
}

// popDue removes and returns every subtrace due at or before now, earliest
// first.
func (d *subtraceDeadlines) popDue(now time.Time) []subtraceID {
	var due []subtraceID
	for len(d.pq) > 0 && !d.pq[0].due.After(now) {
		e := heap.Pop(&d.pq).(*deadlineEntry)
		delete(d.index, e.id)
		due = append(due, e.id)
	}
	return due
}

func (d *subtraceDeadlines) len() int {
	return len(d.pq)
}

// deadlinePQ carries the heap.Interface implementation. Callers use the methods
// on subtraceDeadlines, which keep the index in step with the queue.
type deadlinePQ []*deadlineEntry

func (pq deadlinePQ) Len() int { return len(pq) }

func (pq deadlinePQ) Less(i, j int) bool { return pq[i].due.Before(pq[j].due) }

func (pq deadlinePQ) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].at = i
	pq[j].at = j
}

func (pq *deadlinePQ) Push(x any) {
	e := x.(*deadlineEntry)
	e.at = len(*pq)
	*pq = append(*pq, e)
}

func (pq *deadlinePQ) Pop() any {
	old := *pq
	last := len(old) - 1
	e := old[last]
	old[last] = nil // don't hold the entry alive through the spare capacity
	*pq = old[:last]
	return e
}
