// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package priorityqueue // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/priorityqueue"

import (
	"cmp"
)

type PriorityValueType cmp.Ordered

type QueueItem[V any, P PriorityValueType] struct {
	Value    V
	Priority P
	Index    int
}

type PriorityQueue[V any, P PriorityValueType] []*QueueItem[V, P]

func (pq PriorityQueue[V, P]) Len() int { return len(pq) }

func (pq PriorityQueue[V, P]) Less(i, j int) bool {
	return pq[i].Priority > pq[j].Priority
}

func (pq PriorityQueue[V, P]) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *PriorityQueue[V, P]) Push(x any) {
	n := len(*pq)
	item := x.(*QueueItem[V, P])
	item.Index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue[V, P]) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.Index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
