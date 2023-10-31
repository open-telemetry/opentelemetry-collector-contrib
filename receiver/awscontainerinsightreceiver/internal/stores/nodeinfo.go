// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
)

type nodeStats struct {
	podCnt       int
	containerCnt int
	cpuReq       uint64
	memReq       uint64
}

type nodeInfo struct {
	statsLock sync.RWMutex
	nodeStats nodeStats

	cpuLock     sync.RWMutex
	CPUCapacity uint64

	memLock     sync.RWMutex
	MemCapacity uint64

	logger *zap.Logger
}

func newNodeInfo(logger *zap.Logger) *nodeInfo {
	nc := &nodeInfo{
		logger: logger,
	}
	return nc
}

func (n *nodeInfo) setCPUCapacity(cpuCapacity interface{}) {
	n.cpuLock.Lock()
	defer n.cpuLock.Unlock()
	n.CPUCapacity = forceConvertToInt64(cpuCapacity, n.logger)
}

func (n *nodeInfo) setMemCapacity(memCapacity interface{}) {
	n.memLock.Lock()
	defer n.memLock.Unlock()
	n.MemCapacity = forceConvertToInt64(memCapacity, n.logger)
}

func (n *nodeInfo) getCPUCapacity() uint64 {
	n.cpuLock.RLock()
	defer n.cpuLock.RUnlock()
	return n.CPUCapacity
}

func (n *nodeInfo) getMemCapacity() uint64 {
	n.memLock.RLock()
	defer n.memLock.RUnlock()
	return n.MemCapacity
}

func (n *nodeInfo) setNodeStats(stats nodeStats) {
	n.statsLock.Lock()
	defer n.statsLock.Unlock()
	n.nodeStats = stats
}

func (n *nodeInfo) getNodeStats() nodeStats {
	n.statsLock.RLock()
	defer n.statsLock.RUnlock()
	return n.nodeStats
}

func forceConvertToInt64(v interface{}, logger *zap.Logger) uint64 {
	var value uint64

	switch t := v.(type) {
	case int:
		if t < 0 {
			logger.Error("value is invalid", zap.Any("value", t))
			return 0
		}
		value = uint64(t)
	case int32:
		if t < 0 {
			logger.Error("value is invalid", zap.Any("value", t))
			return 0
		}
		value = uint64(t)
	case int64:
		if t < 0 {
			logger.Error("value is invalid", zap.Any("value", t))
			return 0
		}
		value = uint64(t)
	case uint:
		value = uint64(t)
	case uint32:
		value = uint64(t)
	case uint64:
		value = t
	default:
		logger.Error(fmt.Sprintf("value type does not support: %v, %T", v, v))
		value = 0
	}

	return value
}
