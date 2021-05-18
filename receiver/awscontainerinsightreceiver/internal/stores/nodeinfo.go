// Copyright  OpenTelemetry Authors
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

package stores

import (
	"log"
	"sync"
)

type nodeStats struct {
	podCnt       int
	containerCnt int
	cpuReq       int64
	memReq       int64
}

type nodeInfo struct {
	nodeStats nodeStats
	cpuLock   *sync.RWMutex
	memLock   *sync.RWMutex

	CPUCapacity int64
	MemCapacity int64
}

func newNodeInfo() *nodeInfo {
	nc := &nodeInfo{
		cpuLock: &sync.RWMutex{},
		memLock: &sync.RWMutex{},
	}
	return nc
}

func (n *nodeInfo) setCPUCapacity(cpuCapacity interface{}) {
	n.cpuLock.Lock()
	defer n.cpuLock.Unlock()
	n.CPUCapacity = forceConvertToInt64(cpuCapacity)
}

func (n *nodeInfo) setMemCapacity(memCapacity interface{}) {
	n.memLock.Lock()
	defer n.memLock.Unlock()
	n.MemCapacity = forceConvertToInt64(memCapacity)
}

func (n *nodeInfo) getCPUCapacity() int64 {
	n.cpuLock.RLock()
	defer n.cpuLock.RUnlock()
	return n.CPUCapacity
}

func (n *nodeInfo) getMemCapacity() int64 {
	n.memLock.RLock()
	defer n.memLock.RUnlock()
	return n.MemCapacity
}

func forceConvertToInt64(v interface{}) int64 {
	var value int64

	switch t := v.(type) {
	case int:
		value = int64(t)
	case int32:
		value = int64(t)
	case int64:
		value = t
	case uint:
		value = int64(t)
	case uint32:
		value = int64(t)
	case uint64:
		value = int64(t)
	default:
		log.Printf("value type does not support: %v, %T", v, v)
		value = 0
	}

	if value < 0 {
		log.Printf("value is invalid: %v", value)
		return 0
	}

	return value
}
