// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package priorityqueue

import (
	"container/heap"
	"testing"
)

// Helper function to update an item's priority and re-fix the heap.
// This is a common operation for priority queues that relies on the Index field.
func update[V any, P PriorityValueType](pq *PriorityQueue[V, P], item *QueueItem[V, P], newPriority P) {
	item.Priority = newPriority
	heap.Fix(pq, item.Index)
}

func TestPriorityQueue_Operations(t *testing.T) {
	t.Parallel()

	// --- Test Case 1: String Value, Int Priority (Basic Push/Pop) ---
	t.Run("StringValueIntPriority", func(t *testing.T) {
		pq := make(PriorityQueue[string, int], 0)
		heap.Init(&pq)

		itemsToPush := []*QueueItem[string, int]{
			{Value: "Task A", Priority: 3},
			{Value: "Task B", Priority: 1},
			{Value: "Task C", Priority: 5}, // Highest priority
			{Value: "Task D", Priority: 2},
		}

		for _, item := range itemsToPush {
			heap.Push(&pq, item)
		}

		if pq.Len() != len(itemsToPush) {
			t.Errorf("Expected length %d after pushes, got %d", len(itemsToPush), pq.Len())
		}

		expectedOrder := []string{"Task C", "Task A", "Task D", "Task B"} // Priorities: 5, 3, 2, 1

		for i, expectedValue := range expectedOrder {
			if pq.Len() == 0 {
				t.Fatalf("Queue unexpectedly empty at step %d", i)
			}
			poppedItem := heap.Pop(&pq).(*QueueItem[string, int])
			if poppedItem.Value != expectedValue {
				t.Errorf("Step %d: Expected value %s, got %s", i, expectedValue, poppedItem.Value)
			}
			if poppedItem.Index != -1 { // Verify index is reset for safety
				t.Errorf("Step %d: Expected popped item index to be -1, got %d", i, poppedItem.Index)
			}
		}
		if pq.Len() != 0 {
			t.Errorf("Expected length 0 after popping all items, got %d", pq.Len())
		}
	})

	// --- Test Case 2: Update Priority ---
	t.Run("UpdatePriority", func(t *testing.T) {
		pq := make(PriorityQueue[string, int], 0)
		heap.Init(&pq)

		itemA := &QueueItem[string, int]{Value: "A", Priority: 10}
		itemB := &QueueItem[string, int]{Value: "B", Priority: 20}
		itemC := &QueueItem[string, int]{Value: "C", Priority: 5}

		heap.Push(&pq, itemA)
		heap.Push(&pq, itemB)
		heap.Push(&pq, itemC)

		// Initial state (max-heap): B (20), A (10), C (5)

		// Update itemA's priority to be the highest (25)
		update(&pq, itemA, 25) // A is now 25

		// Expected order: A (25), B (20), C (5)
		expectedOrder := []string{"A", "B", "C"}
		for i, expectedValue := range expectedOrder {
			poppedItem := heap.Pop(&pq).(*QueueItem[string, int])
			if poppedItem.Value != expectedValue {
				t.Errorf("After update (A to 25), step %d: Expected value %s, got %s", i, expectedValue, poppedItem.Value)
			}
		}
		if pq.Len() != 0 {
			t.Errorf("Expected length 0 after update test, got %d", pq.Len())
		}
	})

	// --- Test Case 3: Edge Cases and Index Management ---
	t.Run("EdgeCasesAndIndexManagement", func(t *testing.T) {
		// Test empty queue
		pqEmpty := make(PriorityQueue[string, int], 0)
		heap.Init(&pqEmpty)
		if pqEmpty.Len() != 0 {
			t.Errorf("Expected empty queue length 0, got %d", pqEmpty.Len())
		}

		// Test single item
		pqSingle := make(PriorityQueue[string, int], 0)
		heap.Init(&pqSingle)
		singleItem := &QueueItem[string, int]{Value: "Single", Priority: 10}
		heap.Push(&pqSingle, singleItem)
		if pqSingle.Len() != 1 {
			t.Errorf("Expected single item queue length 1, got %d", pqSingle.Len())
		}
		popped := heap.Pop(&pqSingle).(*QueueItem[string, int])
		if popped.Value != "Single" || popped.Priority != 10 {
			t.Errorf("Expected single item {Value: Single, Priority: 10}, got %+v", popped)
		}
		if pqSingle.Len() != 0 {
			t.Errorf("Expected single item queue length 0 after pop, got %d", pqSingle.Len())
		}

		// Test index management during update
		pqIndex := make(PriorityQueue[string, int], 0)
		heap.Init(&pqIndex)
		itemA := &QueueItem[string, int]{Value: "A", Priority: 10}
		itemB := &QueueItem[string, int]{Value: "B", Priority: 20}
		heap.Push(&pqIndex, itemA)
		heap.Push(&pqIndex, itemB)

		// After pushes, itemA and itemB will have some indices.
		// When itemA's priority is increased, it should move to the top (index 0).
		update(&pqIndex, itemA, 30) // A is now highest

		if pqIndex[0] != itemA {
			t.Errorf("Expected itemA to be at index 0 after update to highest priority, but it's not.")
		}
		if itemA.Index != 0 {
			t.Errorf("Expected itemA.Index to be 0 after update to highest priority, got %d", itemA.Index)
		}

		// Pop and verify index reset
		poppedItem := heap.Pop(&pqIndex).(*QueueItem[string, int])
		if poppedItem.Index != -1 {
			t.Errorf("Expected popped item's index to be -1, got %d", poppedItem.Index)
		}
	})

	// --- Test Case 5: Direct Less/Swap Functionality ---
	t.Run("LessAndSwapFunctions", func(t *testing.T) {
		pq := make(PriorityQueue[string, int], 2)
		item1 := &QueueItem[string, int]{Value: "First", Priority: 10, Index: 0}
		item2 := &QueueItem[string, int]{Value: "Second", Priority: 20, Index: 1}
		pq[0] = item1
		pq[1] = item2

		// Test Less: For a max-heap, Less(i, j) is true if pq[i].Priority < pq[j].Priority
		// This means the item at 'i' has *lower* priority than 'j', so 'j' should come before 'i'.
		// Our Less function is `pq[i].Priority > pq[j].Priority` for max-heap.
		// So, Less(0, 1) means 10 > 20, which is false.
		if pq.Less(0, 1) {
			t.Errorf("Less(0, 1) expected false (10 not greater than 20), got true")
		}

		// Test Swap
		pq.Swap(0, 1)
		// Verify items are swapped
		if pq[0] != item2 || pq[1] != item1 {
			t.Errorf("Items not swapped correctly. Expected pq[0]=item2, pq[1]=item1. Got pq[0]=%+v, pq[1]=%+v", pq[0], pq[1])
		}
		// Verify indices are updated
		if item1.Index != 1 {
			t.Errorf("item1.Index expected 1, got %d", item1.Index)
		}
		if item2.Index != 0 {
			t.Errorf("item2.Index expected 0, got %d", item2.Index)
		}
	})
}
