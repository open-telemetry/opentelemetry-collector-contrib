// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension"

import (
	"context"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
)

// inMemoryAckExtension is the in-memory implementation of the AckExtension
// When MaxNumPartition is reached, the acks associated with the least recently used partition are evicted.
// When MaxNumPendingAcksPerPartition is reached, the least recently used ack is evicted
type inMemoryAckExtension struct {
	partitionMap                  *lru.Cache[string, *ackPartition]
	maxNumPendingAcksPerPartition uint64
}

func newInMemoryAckExtension(conf *Config) *inMemoryAckExtension {
	cache, _ := lru.New[string, *ackPartition](int(conf.MaxNumPartition))
	return &inMemoryAckExtension{
		partitionMap:                  cache,
		maxNumPendingAcksPerPartition: conf.MaxNumPendingAcksPerPartition,
	}
}

type ackPartition struct {
	id     atomic.Uint64
	ackMap *lru.Cache[uint64, bool]
}

func newAckPartition(maxPendingAcks uint64) *ackPartition {
	cache, _ := lru.New[uint64, bool](int(maxPendingAcks))
	return &ackPartition{
		ackMap: cache,
	}
}

func (as *ackPartition) nextAck() uint64 {
	id := as.id.Add(1)
	as.ackMap.Add(id, false)
	return id
}

func (as *ackPartition) ack(key uint64) {
	if _, ok := as.ackMap.Get(key); ok {
		as.ackMap.Add(key, true)
	}
}

func (as *ackPartition) computeAcks(ackIDs []uint64) map[uint64]bool {
	result := make(map[uint64]bool, len(ackIDs))
	for _, val := range ackIDs {
		if isAcked, ok := as.ackMap.Get(val); ok && isAcked {
			result[val] = true
			as.ackMap.Remove(val)
		} else {
			result[val] = false
		}
	}

	return result
}

// Start of inMemoryAckExtension does nothing and returns nil
func (i *inMemoryAckExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown of inMemoryAckExtension does nothing and returns nil
func (i *inMemoryAckExtension) Shutdown(_ context.Context) error {
	return nil
}

// ProcessEvent marks the beginning of processing an event. It generates an ack ID for the associated partition ID.
func (i *inMemoryAckExtension) ProcessEvent(partitionID string) (ackID uint64) {
	if val, ok := i.partitionMap.Get(partitionID); ok {
		return val.nextAck()
	}

	i.partitionMap.ContainsOrAdd(partitionID, newAckPartition(i.maxNumPendingAcksPerPartition))
	val, _ := i.partitionMap.Get(partitionID)
	return val.nextAck()
}

// Ack acknowledges an event has been processed.
func (i *inMemoryAckExtension) Ack(partitionID string, ackID uint64) {
	if val, ok := i.partitionMap.Get(partitionID); ok {
		val.ack(ackID)
	}
}

// QueryAcks checks the statuses of given ackIDs for a partition.
// ackIDs that are not generated from ProcessEvent or have been removed as a result of previous calls to QueryAcks will return false.
func (i *inMemoryAckExtension) QueryAcks(partitionID string, ackIDs []uint64) map[uint64]bool {
	if val, ok := i.partitionMap.Get(partitionID); ok {
		return val.computeAcks(ackIDs)
	}

	result := make(map[uint64]bool, len(ackIDs))
	for _, ackID := range ackIDs {
		result[ackID] = false
	}

	return result
}
