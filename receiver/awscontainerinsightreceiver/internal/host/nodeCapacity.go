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

package host

import (
	"os"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"go.uber.org/zap"
)

const (
	GoPSUtilProcDirEnv = "HOST_PROC"
)

type NodeCapacity struct {
	MemCapacity int64
	CPUCapacity int64
	logger      *zap.Logger
}

func NewNodeCapacity(logger *zap.Logger) (*NodeCapacity, error) {
	if _, err := os.Lstat("/rootfs/proc"); os.IsNotExist(err) {
		return nil, err
	}
	if err := os.Setenv(GoPSUtilProcDirEnv, "/rootfs/proc"); err != nil {
		logger.Error("NodeCapacity cannot set goPSUtilProcDirEnv to /rootfs/proc", zap.Error(err))
	}
	nc := &NodeCapacity{logger: logger}
	nc.parseCPU()
	nc.parseMemory()
	return nc, nil
}

func (n *NodeCapacity) parseMemory() {
	if memStats, err := mem.VirtualMemory(); err == nil {
		n.MemCapacity = int64(memStats.Total)
	} else {
		// If any error happen, then there will be no mem utilization metrics
		n.logger.Error("NodeCapacity cannot get memStats from psUtil", zap.Error(err))
	}
}

func (n *NodeCapacity) parseCPU() {
	if cpuInfos, err := cpu.Info(); err == nil {
		numCores := len(cpuInfos)
		n.CPUCapacity = int64(numCores)
	} else {
		// If any error happen, then there will be no cpu utilization metrics
		n.logger.Error("NodeCapacity cannot get cpuInfo from psUtil", zap.Error(err))
	}
}
