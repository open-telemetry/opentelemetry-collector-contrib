// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension

import (
	"fmt"
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

	for k, v := range map2 {
		map1[k] = v
	}

	require.Equal(t, len(map1), ackSize)
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
			ext.Ack(fmt.Sprintf("part-%d", i), 1)
		} else {
			ext.Ack(fmt.Sprintf("part-%d", i), 0)
			ext.Ack(fmt.Sprintf("part-%d", i), 2)
		}
	}

	// second event of even partitions should be acked, and first and third events of odd partitions should be acked
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[0], false)
			require.Equal(t, result[1], true)
			require.Equal(t, result[2], false)
		} else {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[0], true)
			require.Equal(t, result[1], false)
			require.Equal(t, result[2], true)
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
			ext.Ack(fmt.Sprintf("part-%d", i), 1)
		} else {
			ext.Ack(fmt.Sprintf("part-%d", i), 0)
			ext.Ack(fmt.Sprintf("part-%d", i), 2)
		}
	}

	// second event of even partitions should be acked, and first and third events of odd partitions should be acked
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[0], false)
			require.Equal(t, result[1], true)
			require.Equal(t, result[2], false)
		} else {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[0], true)
			require.Equal(t, result[1], false)
			require.Equal(t, result[2], true)
		}
	}

	// querying the same acked events should result in false
	for i := 0; i < 100; i++ {
		result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
		require.Equal(t, len(result), 3)
		require.Equal(t, result[0], false)
		require.Equal(t, result[1], false)
		require.Equal(t, result[2], false)
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
		result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
		require.Equal(t, len(result), 3)
		require.Equal(t, result[0], false)
		require.Equal(t, result[1], false)
		require.Equal(t, result[2], false)
	}

	wg.Add(partitionCount)
	// ack the second event of all even partitions and first and third events of all odd partitions
	for i := 0; i < partitionCount; i++ {
		i := i
		go func() {
			if i%2 == 0 {
				ext.Ack(fmt.Sprintf("part-%d", i), 1)
			} else {
				ext.Ack(fmt.Sprintf("part-%d", i), 0)
				ext.Ack(fmt.Sprintf("part-%d", i), 2)
			}
			wg.Done()
		}()
	}

	wg.Wait()
	// second event of even partitions should be acked, and first and third events of odd partitions should be acked
	for i := 0; i < partitionCount; i++ {
		if i%2 == 0 {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[0], false)
			require.Equal(t, result[1], true)
			require.Equal(t, result[2], false)
		} else {
			result := ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
			require.Equal(t, len(result), 3)
			require.Equal(t, result[0], true)
			require.Equal(t, result[1], false)
			require.Equal(t, result[2], true)
		}
	}
	wg.Add(100)
	resultChan := make(chan map[uint64]bool, partitionCount)
	// querying the same acked events should result in false
	for i := 0; i < partitionCount; i++ {
		i := i
		go func() {
			resultChan <- ext.QueryAcks(fmt.Sprintf("part-%d", i), []uint64{0, 1, 2})
			wg.Done()
		}()
	}
	wg.Wait()

	for i := 0; i < partitionCount; i++ {
		result := <-resultChan
		require.Equal(t, len(result), 3)
		require.Equal(t, result[0], false)
		require.Equal(t, result[1], false)
		require.Equal(t, result[2], false)
	}
}
