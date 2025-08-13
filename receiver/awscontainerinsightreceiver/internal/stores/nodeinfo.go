// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
)

type nodeStats struct {
	podCnt               int
	containerCnt         int
	cpuReq               uint64
	memReq               uint64
	gpuReq               uint64
	gpuUsageTotal        uint64
	neuroncoreReq        uint64
	neuroncoreUsageTotal uint64
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
	NodeToLabelsMap() map[string]map[k8sclient.Label]int8
}

func newNodeInfo(nodeName string, provider nodeInfoProvider, logger *zap.Logger) *nodeInfo {
	nc := &nodeInfo{
		nodeName: nodeName,
		provider: provider,
		logger:   logger,
	}
	return nc
}

func (n *nodeInfo) setCPUCapacity(cpuCapacity any) {
	n.cpuLock.Lock()
	defer n.cpuLock.Unlock()
	n.CPUCapacity = forceConvertToInt64(cpuCapacity, n.logger)
}

func (n *nodeInfo) setMemCapacity(memCapacity any) {
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

func (n *nodeInfo) getNodeStatusCapacityGPUs() (uint64, bool) {
	capacityResources, ok := n.provider.NodeToCapacityMap()[n.nodeName]
	if !ok {
		return 0, false
	}
	gpus := capacityResources.Name(resourceSpecNvidiaGpuKey, resource.DecimalExponent).Value()
	return forceConvertToInt64(gpus, n.logger), true
}

func (n *nodeInfo) getNeuronResourceCapacity(resourceKey v1.ResourceName) (uint64, bool) {
	capacityResources, ok := n.provider.NodeToCapacityMap()[n.nodeName]
	if !ok {
		return 0, false
	}
	value := capacityResources.Name(resourceKey, resource.DecimalExponent).Value()
	return forceConvertToInt64(value, n.logger), true
}

func (n *nodeInfo) getNodeStatusCapacityNeuron() (uint64, bool) {
	return n.getNeuronResourceCapacity(resourceSpecNeuronKey)
}

func (n *nodeInfo) getNodeStatusCapacityNeuroncore() (uint64, bool) {
	return n.getNeuronResourceCapacity(resourceSpecNeuroncoreKey)
}

func (n *nodeInfo) getNodeStatusCapacityNeurondevice() (uint64, bool) {
	return n.getNeuronResourceCapacity(resourceSpecNeuronDeviceKey)
}

func (n *nodeInfo) getNeuronCoresPerDevice() (int, bool) {
	devices, hasDevices := n.getNodeStatusCapacityNeuron()
	cores, hasCores := n.getNodeStatusCapacityNeuroncore()

	if hasDevices && hasCores && devices > 0 && cores > 0 {
		return int(cores / devices), true
	}

	return 0, false
}

func (n *nodeInfo) getNodeStatusCapacityNeuronCores() (uint64, bool) {
	if cores, ok := n.getNodeStatusCapacityNeuroncore(); ok {
		return cores, true
	}

	coresPerDevice, hasRatio := n.getNeuronCoresPerDevice()
	if !hasRatio {
		return 0, false
	}

	if devices, ok := n.getNodeStatusCapacityNeuron(); ok {
		return devices * uint64(coresPerDevice), true
	}
	if devices, ok := n.getNodeStatusCapacityNeurondevice(); ok {
		return devices * uint64(coresPerDevice), true
	}

	return 0, false
}

func (n *nodeInfo) getNodeStatusCondition(conditionType v1.NodeConditionType) (uint64, bool) {
	if nodeConditions, ok := n.provider.NodeToConditionsMap()[n.nodeName]; ok {
		if conditionStatus, ok := nodeConditions[conditionType]; ok {
			if conditionStatus == v1.ConditionTrue {
				return 1, true
			}
			return 0, true // v1.ConditionFalse or v1.ConditionUnknown
		}
	}
	return 0, false
}

func (n *nodeInfo) getNodeConditionUnknown() (uint64, bool) {
	if nodeConditions, ok := n.provider.NodeToConditionsMap()[n.nodeName]; ok {
		for _, conditionStatus := range nodeConditions {
			if conditionStatus == v1.ConditionUnknown {
				return 1, true
			}
		}
		return 0, true
	}
	return 0, false
}

func forceConvertToInt64(v any, logger *zap.Logger) uint64 {
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
