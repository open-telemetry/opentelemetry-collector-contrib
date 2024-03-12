// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension"

import (
	"context"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
)

type InMemoryAckExtension struct {
	partitionMap                  *lru.Cache[string, *ackStatus]
	maxNumPendingAcksPerPartition uint64
}

func newInMemoryAckExtension(conf *Config) *InMemoryAckExtension {
	cache, _ := lru.New[string, *ackStatus](int(conf.MaxNumPartition))
	return &InMemoryAckExtension{
		partitionMap:                  cache,
		maxNumPendingAcksPerPartition: defaultMaxNumPendingAcksPerPartition,
	}
}

type ackStatus struct {
	id     atomic.Uint64
	ackMap *lru.Cache[uint64, bool]
}

func newAckStatus(maxPendingAcks uint64) *ackStatus {
	id := uint64(0)

	cache, _ := lru.New[uint64, bool](int(maxPendingAcks))
	cache.Add(id, false)

	as := ackStatus{
		ackMap: cache,
	}
	as.id.Store(id)
	return &as
}

func (as *ackStatus) nextAck() uint64 {
	as.ackMap.Add(as.id.Add(1), false)
	return as.id.Load()
}

func (as *ackStatus) ack(key uint64) {
	if _, ok := as.ackMap.Get(key); ok {
		as.ackMap.Add(key, true)
	}
}

func (as *ackStatus) computeAcks(ackIDs []uint64) map[uint64]bool {
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

// Start of InMemoryAckExtension does nothing and returns nil
func (i *InMemoryAckExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown of InMemoryAckExtension does nothing and returns nil
func (i *InMemoryAckExtension) Shutdown(_ context.Context) error {
	return nil
}

// ProcessEvent marks the beginning of processing an event. It generates an ack ID for the associated partition ID.
func (i *InMemoryAckExtension) ProcessEvent(partitionID string) (ackID uint64) {
	if val, ok := i.partitionMap.Get(partitionID); ok {
		return val.nextAck()
	}

	i.partitionMap.Add(partitionID, newAckStatus(i.maxNumPendingAcksPerPartition))
	return 0
}

// Ack acknowledges an event has been processed.
func (i *InMemoryAckExtension) Ack(partitionID string, ackID uint64) {
	if val, ok := i.partitionMap.Get(partitionID); ok {
		val.ack(ackID)
	}
}

// QueryAcks checks the statuses of given ackIDs for a partition.
func (i *InMemoryAckExtension) QueryAcks(partitionID string, ackIDs []uint64) map[uint64]bool {
	if val, ok := i.partitionMap.Get(partitionID); ok {
		return val.computeAcks(ackIDs)
	}

	result := make(map[uint64]bool, len(ackIDs))
	for _, ackID := range ackIDs {
		result[ackID] = false
	}

	return result
}
