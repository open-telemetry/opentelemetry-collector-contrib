// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension"

import (
	"context"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
)

type InMemoryAckExtension struct {
	partitionMap sync.Map
}

func newInMemoryAckExtension() *InMemoryAckExtension {
	return &InMemoryAckExtension{}
}

type ackStatus struct {
	id     atomic.Uint64
	ackMap sync.Map
}

func newAckStatus() *ackStatus {
	id := uint64(0)
	as := ackStatus{}
	as.id.Store(id)
	as.ackMap.Store(id, false)
	return &as
}

func (as *ackStatus) nextAck() uint64 {
	as.ackMap.Store(as.id.Add(1), false)
	return as.id.Load()
}

func (as *ackStatus) ack(key uint64) {
	if _, ok := as.ackMap.Load(key); ok {
		as.ackMap.Store(key, true)
	}
}

func (as *ackStatus) computeAcks(ackIDs []uint64) map[uint64]bool {
	result := make(map[uint64]bool, len(ackIDs))
	for _, val := range ackIDs {
		if isAcked, ok := as.ackMap.Load(val); ok && isAcked.(bool) {
			result[val] = true
			as.ackMap.Delete(val)
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
	if actual, loaded := i.partitionMap.LoadOrStore(partitionID, newAckStatus()); loaded {
		status := actual.(*ackStatus)
		return status.nextAck()
	}

	return 0
}

// Ack acknowledges an event has been processed.
func (i *InMemoryAckExtension) Ack(partitionID string, ackID uint64) {
	if val, ok := i.partitionMap.Load(partitionID); ok {
		if status, ok := val.(*ackStatus); ok {
			status.ack(ackID)
		}
	}
}

// QueryAcks checks the statuses of given ackIDs for a partition.
func (i *InMemoryAckExtension) QueryAcks(partitionID string, ackIDs []uint64) map[uint64]bool {
	if val, ok := i.partitionMap.Load(partitionID); ok {
		if status, ok := val.(*ackStatus); ok {
			return status.computeAcks(ackIDs)
		}
	}

	result := make(map[uint64]bool, len(ackIDs))
	for _, ackID := range ackIDs {
		result[ackID] = false
	}

	return result
}
