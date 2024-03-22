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
		for i := 0; i < 100; i++ {
			// each partition has 3 events
			map1[ext.ProcessEvent(fmt.Sprint(partitionName))] = struct{}{}
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			// each partition has 3 events
			map2[ext.ProcessEvent(fmt.Sprint(partitionName))] = struct{}{}
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			// each partition has 3 events
			map3[ext.ProcessEvent(fmt.Sprint(partitionName))] = struct{}{}
		}
		wg.Done()
	}()

	wg.Wait()

	maps.Copy(map1, map2)
	maps.Copy(map1, map3)

	require.Equal(t, len(map1), 300)
}

func TestExtensionAck_ProcessEvents_EventsUnAcked(t *testing.T) {
	conf := Config{
		MaxNumPartition:               defaultMaxNumPartition,
		MaxNumPendingAcksPerPartition: defaultMaxNumPendingAcksPerPartition,
	}
	ext := newInMemoryAckExtension(&conf)

	// send events through different partitions
	for i := 0; i < 100; i++ {
		// each partition has 3 events
		for j := 0; j < 3; j++ {
			ext.ProcessEvent(fmt.Sprintf("part-%d", i))
		}
	}

	// non-acked events should be return false
	for i := 0; i < 100; i++ {
		result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
		require.Equal(t, len(result), 3)
		require.Equal(t, result[0], false)
		require.Equal(t, result[1], false)
		require.Equal(t, result[2], false)
	}
}

func TestExtensionAck_ProcessEvents_EventsAcked(t *testing.T) {
	conf := Config{
		MaxNumPartition:               defaultMaxNumPartition,
		MaxNumPendingAcksPerPartition: defaultMaxNumPendingAcksPerPartition,
	}
	ext := newInMemoryAckExtension(&conf)

	// send events through different partitions
	for i := 0; i < 100; i++ {
		// each partition has 3 events
		for j := 0; j < 3; j++ {
			ext.ProcessEvent(fmt.Sprintf("part-%d", i))
		}
	}

	// ack the second event of all even partitions and first and third events of all odd partitions
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			ext.Ack(fmt.Sprintf("part-%d", i), 2)
		} else {
			ext.Ack(fmt.Sprintf("part-%d", i), 1)
			ext.Ack(fmt.Sprintf("part-%d", i), 3)
		}
	}

	// second event of even partitions should be acked, and first and third events of odd partitions should be acked
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[1], false)
			require.Equal(t, result[2], true)
			require.Equal(t, result[3], false)
		} else {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[1], true)
			require.Equal(t, result[2], false)
			require.Equal(t, result[3], true)
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
	for i := 0; i < 100; i++ {
		// each partition has 3 events
		for j := 0; j < 3; j++ {
			ext.ProcessEvent(fmt.Sprintf("part-%d", i))
		}
	}

	// ack the second event of all even partitions and first and third events of all odd partitions
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			ext.Ack(fmt.Sprintf("part-%d", i), 2)
		} else {
			ext.Ack(fmt.Sprintf("part-%d", i), 1)
			ext.Ack(fmt.Sprintf("part-%d", i), 3)
		}
	}

	// second event of even partitions should be acked, and first and third events of odd partitions should be acked
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[1], false)
			require.Equal(t, result[2], true)
			require.Equal(t, result[3], false)
		} else {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[1], true)
			require.Equal(t, result[2], false)
			require.Equal(t, result[3], true)
		}
	}

	// querying the same acked events should result in false
	for i := 0; i < 100; i++ {
		result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
		require.Equal(t, len(result), 3)
		require.Equal(t, result[1], false)
		require.Equal(t, result[2], false)
		require.Equal(t, result[3], false)
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
	for i := 0; i < partitionCount; i++ {
		i := i
		go func() {
			// each partition has 3 events
			for j := 0; j < 3; j++ {
				ext.ProcessEvent(fmt.Sprintf("part-%d", i))
			}
			wg.Done()
		}()
	}

	wg.Wait()

	// non-acked events should be return false
	for i := 0; i < partitionCount; i++ {
		result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
		require.Equal(t, len(result), 3)
		require.Equal(t, result[1], false)
		require.Equal(t, result[2], false)
		require.Equal(t, result[3], false)
	}

	wg.Add(partitionCount)
	// ack the second event of all even partitions and first and third events of all odd partitions
	for i := 0; i < partitionCount; i++ {
		i := i
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
	for i := 0; i < partitionCount; i++ {
		if i%2 == 0 {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[1], false)
			require.Equal(t, result[2], true)
			require.Equal(t, result[3], false)
		} else {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[1], true)
			require.Equal(t, result[2], false)
			require.Equal(t, result[3], true)
		}
	}
	wg.Add(100)
	resultChan := make(chan map[uint64]bool, partitionCount)
	// querying the same acked events should result in false
	for i := 0; i < partitionCount; i++ {
		i := i
		go func() {
			resultChan <- ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{1, 2, 3})
			wg.Done()
		}()
	}
	wg.Wait()

	for i := 0; i < partitionCount; i++ {
		result := <-resultChan
		require.Equal(t, len(result), 3)
		require.Equal(t, result[1], false)
		require.Equal(t, result[2], false)
		require.Equal(t, result[3], false)
	}
}
