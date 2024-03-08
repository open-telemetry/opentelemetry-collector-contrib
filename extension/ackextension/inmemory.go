package ackextension

import (
	"sync"
)

type InMemoryAckExtension struct {
	partitionMap sync.Map
}

func NewInMemoryAckExtension() AckExtension {
	return &InMemoryAckExtension{}
}

type ackStatus struct {
	id     uint64
	ackMap sync.Map
}

func newAckStatus() *ackStatus {
	id := uint64(0)
	ret := ackStatus{id: id}
	ret.ackMap.Store(id, false)
	return &ret
}

func (as *ackStatus) nextAck() uint64 {
	as.id += 1
	as.ackMap.Store(as.id, false)
	return as.id
}

func (as *ackStatus) ack(key uint64) {
	if _, ok := as.ackMap.Load(key); ok {
		as.ackMap.Store(key, true)
	}
}

func (as *ackStatus) queryAcks(ackIDs []uint64) map[uint64]bool {
	result := map[uint64]bool{}
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

func (i *InMemoryAckExtension) ProcessEvent(partitionID string) (ackID uint64) {
	if actual, loaded := i.partitionMap.LoadOrStore(partitionID, newAckStatus()); loaded {
		ackStats := actual.(*ackStatus)
		return ackStats.nextAck()
	}

	return 0
}

func (i *InMemoryAckExtension) Ack(partitionID string, ackID uint64) {
	if val, ok := i.partitionMap.Load(partitionID); ok {
		if ackStats, ok := val.(*ackStatus); ok {
			ackStats.ack(ackID)
		}
	}
}

func (i *InMemoryAckExtension) QueryAcks(partitionID string, ackIDs []uint64) map[uint64]bool {
	if val, ok := i.partitionMap.Load(partitionID); ok {
		if ackStats, ok := val.(*ackStatus); ok {
			return ackStats.queryAcks(ackIDs)
		}
	}

	result := map[uint64]bool{}
	for _, ackID := range ackIDs {
		result[ackID] = false
	}

	return result
}
