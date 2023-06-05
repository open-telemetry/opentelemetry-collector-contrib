// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

type nodeStats struct {
	podCnt       int
	containerCnt int
	cpuReq       uint64
	memReq       uint64
}

type nodeInfo struct {
	nodeName string
	provider nodeInfoProvider

	statsLock sync.RWMutex
	nodeStats nodeStats

	cpuLock     sync.RWMutex
	CPUCapacity uint64

	memLock     sync.RWMutex
	MemCapacity uint64

	logger *zap.Logger
}

type nodeInfoProvider interface {
	NodeToCapacityMap() map[string]v1.ResourceList
	NodeToAllocatableMap() map[string]v1.ResourceList
	NodeToConditionsMap() map[string]map[v1.NodeConditionType]v1.ConditionStatus
}

func newNodeInfo(nodeName string, provider nodeInfoProvider, logger *zap.Logger) *nodeInfo {
	nc := &nodeInfo{
		nodeName: nodeName,
		provider: provider,
		logger:   logger,
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

func (n *nodeInfo) getNodeStatusCapacityPods() (uint64, bool) {
	capacityResources, ok := n.provider.NodeToCapacityMap()[n.nodeName]
	if !ok {
		return 0, false
	}
	pods, _ := capacityResources.Pods().AsInt64()
	return forceConvertToInt64(pods, n.logger), true
}

func (n *nodeInfo) getNodeStatusAllocatablePods() (uint64, bool) {
	allocatableResources, ok := n.provider.NodeToAllocatableMap()[n.nodeName]
	if !ok {
		return 0, false
	}
	pods, _ := allocatableResources.Pods().AsInt64()
	return forceConvertToInt64(pods, n.logger), true
}

func (n *nodeInfo) getNodeStatusCondition(conditionType v1.NodeConditionType) (uint64, bool) {
	if nodeConditions, ok := n.provider.NodeToConditionsMap()[n.nodeName]; ok {
		if conditionType, ok := nodeConditions[conditionType]; ok {
			if conditionType == v1.ConditionTrue {
				return 1, true
			}
			return 0, true // v1.ConditionFalse or v1.ConditionUnknown
		}
	}
	return 0, false
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
