// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension

import (
	"fmt"
	"maps"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAckPartitionNextAckConcurrency(t *testing.T) {
	ackSize := 1_000_000
	ap := newAckPartition(uint64(ackSize))
	map1 := map[uint64]struct{}{}
	map2 := map[uint64]struct{}{}
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		for i := 0; i < ackSize/2; i++ {
			map1[ap.nextAck()] = struct{}{}
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < ackSize/2; i++ {
			map2[ap.nextAck()] = struct{}{}
		}
		wg.Done()
	}()

	wg.Wait()

	maps.Copy(map1, map2)
	require.Equal(t, len(map1), ackSize)
}

func TestExtensionAck_ProcessEvents_Concurrency(t *testing.T) {
	partitionName := "partition-name"
	conf := Config{
		MaxNumPartition:               defaultMaxNumPartition,
		MaxNumPendingAcksPerPartition: defaultMaxNumPendingAcksPerPartition,
	}
	ext := newInMemoryAckExtension(&conf)

	var wg sync.WaitGroup
	wg.Add(3)

	map1 := map[uint64]struct{}{}
	map2 := map[uint64]struct{}{}
	map3 := map[uint64]struct{}{}

	// send events through different partitions
	go func() {
		for range 100 {
			// each partition has 3 events
			map1[ext.ProcessEvent(partitionName)] = struct{}{}
		}
		wg.Done()
	}()

	go func() {
		for range 100 {
			// each partition has 3 events
			map2[ext.ProcessEvent(partitionName)] = struct{}{}
		}
		wg.Done()
	}()

	go func() {
		for range 100 {
			// each partition has 3 events
			map3[ext.ProcessEvent(partitionName)] = struct{}{}
		}
		wg.Done()
	}()

	wg.Wait()

	maps.Copy(map1, map2)
	maps.Copy(map1, map3)

	require.Len(t, map1, 300)
}

func TestExtensionAck_ProcessEvents_EventsUnAcked(t *testing.T) {
	conf := Config{
		MaxNumPartition:               defaultMaxNumPartition,
		MaxNumPendingAcksPerPartition: defaultMaxNumPendingAcksPerPartition,
	}
	ext := newInMemoryAckExtension(&conf)

	// send events through different partitions
	for i := range 100 {
		// each partition has 3 events
		for range 3 {
			ext.ProcessEvent(fmt.Sprintf("part-%d", i))
		}
	}

	// non-acked events should be return false
	for i := range 100 {
		result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
		require.Len(t, result, 3)
		require.False(t, result[0])
		require.False(t, result[1])
		require.False(t, result[2])
	}
}

func TestExtensionAck_ProcessEvents_EventsAcked(t *testing.T) {
	conf := Config{
		MaxNumPartition:               defaultMaxNumPartition,
		MaxNumPendingAcksPerPartition: defaultMaxNumPendingAcksPerPartition,
	}
	ext := newInMemoryAckExtension(&conf)

	// send events through different partitions
	for i := range 100 {
		// each partition has 3 events
		for range 3 {
			ext.ProcessEvent(fmt.Sprintf("part-%d", i))
		}
	}

	// ack the second event of all even partitions and first and third events of all odd partitions
	for i := range 100 {
		if i%2 == 0 {
			ext.Ack(fmt.Sprintf("part-%d", i), 2)
		} else {
			ext.Ack(fmt.Sprintf("part-%d", i), 1)
			ext.Ack(fmt.Sprintf("part-%d", i), 3)
		}
	}

	// second event of even partitions should be acked, and first and third events of odd partitions should be acked
	for i := range 100 {
		if i%2 == 0 {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Len(t, result, 3)
			require.False(t, result[1])
			require.True(t, result[2])
			require.False(t, result[3])
		} else {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Len(t, result, 3)
			require.True(t, result[1])
			require.False(t, result[2])
			require.True(t, result[3])
		}
	}
}

func TestExtensionAck_QueryAcks_Unidempotent(t *testing.T) {
	conf := Config{
		MaxNumPartition:               defaultMaxNumPartition,
		MaxNumPendingAcksPerPartition: defaultMaxNumPendingAcksPerPartition,
	}
	ext := newInMemoryAckExtension(&conf)

	// send events through different partitions
	for i := range 100 {
		// each partition has 3 events
		for range 3 {
			ext.ProcessEvent(fmt.Sprintf("part-%d", i))
		}
	}

	// ack the second event of all even partitions and first and third events of all odd partitions
	for i := range 100 {
		if i%2 == 0 {
			ext.Ack(fmt.Sprintf("part-%d", i), 2)
		} else {
			ext.Ack(fmt.Sprintf("part-%d", i), 1)
			ext.Ack(fmt.Sprintf("part-%d", i), 3)
		}
	}

	// second event of even partitions should be acked, and first and third events of odd partitions should be acked
	for i := range 100 {
		if i%2 == 0 {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Len(t, result, 3)
			require.False(t, result[1])
			require.True(t, result[2])
			require.False(t, result[3])
		} else {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Len(t, result, 3)
			require.True(t, result[1])
			require.False(t, result[2])
			require.True(t, result[3])
		}
	}

	// querying the same acked events should result in false
	for i := range 100 {
		result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
		require.Len(t, result, 3)
		require.False(t, result[1])
		require.False(t, result[2])
		require.False(t, result[3])
	}
}

func TestExtensionAckAsync(t *testing.T) {
	conf := Config{
		MaxNumPartition:               defaultMaxNumPartition,
		MaxNumPendingAcksPerPartition: defaultMaxNumPendingAcksPerPartition,
	}
	ext := newInMemoryAckExtension(&conf)

	partitionCount := 100
	var wg sync.WaitGroup
	wg.Add(partitionCount)
	// send events through different partitions
	for i := range partitionCount {
		go func() {
			// each partition has 3 events
			for range 3 {
				ext.ProcessEvent(fmt.Sprintf("part-%d", i))
			}
			wg.Done()
		}()
	}

	wg.Wait()

	// non-acked events should be return false
	for i := range partitionCount {
		result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
		require.Len(t, result, 3)
		require.False(t, result[1])
		require.False(t, result[2])
		require.False(t, result[3])
	}

	wg.Add(partitionCount)
	// ack the second event of all even partitions and first and third events of all odd partitions
	for i := range partitionCount {
		go func() {
			if i%2 == 0 {
				ext.Ack(fmt.Sprintf("part-%d", i), 2)
			} else {
				ext.Ack(fmt.Sprintf("part-%d", i), 1)
				ext.Ack(fmt.Sprintf("part-%d", i), 3)
			}
			wg.Done()
		}()
	}

	wg.Wait()
	// second event of even partitions should be acked, and first and third events of odd partitions should be acked
	for i := range partitionCount {
		if i%2 == 0 {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Len(t, result, 3)
			require.False(t, result[1])
			require.True(t, result[2])
			require.False(t, result[3])
		} else {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Len(t, result, 3)
			require.True(t, result[1])
			require.False(t, result[2])
			require.True(t, result[3])
		}
	}
	wg.Add(100)
	resultChan := make(chan map[uint64]bool, partitionCount)
	// querying the same acked events should result in false
	for i := range partitionCount {
		go func() {
			resultChan <- ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			wg.Done()
		}()
	}
	wg.Wait()

	for range partitionCount {
		result := <-resultChan
		require.Len(t, result, 3)
		require.False(t, result[1])
		require.False(t, result[2])
		require.False(t, result[3])
	}
}
