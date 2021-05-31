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
	"fmt"
	"os"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"go.uber.org/zap"
)

const (
	goPSUtilProcDirEnv = "HOST_PROC"
)

type NodeCapacityProvider interface {
	GetMemoryCapacity() int64
	GetNumCores() int64
}

type NodeCapacity struct {
	MemCapacity int64
	CPUCapacity int64
	logger      *zap.Logger

	// osLstat returns a FileInfo describing the named file.
	osLstat func(name string) (os.FileInfo, error)
	// osSetenv sets the value of the environment variable named by the key
	osSetenv      func(key string, value string) error
	virtualMemory func() (*mem.VirtualMemoryStat, error)
	cpuInfo       func() ([]cpu.InfoStat, error)
}

type nodeCapacityOption func(*NodeCapacity)

func NewNodeCapacity(logger *zap.Logger, options ...nodeCapacityOption) (NodeCapacityProvider, error) {
	nc := &NodeCapacity{
		logger:        logger,
		osLstat:       os.Lstat,
		osSetenv:      os.Setenv,
		virtualMemory: mem.VirtualMemory,
		cpuInfo:       cpu.Info,
	}

	for _, opt := range options {
		opt(nc)
	}

	if _, err := nc.osLstat("/rootfs/proc"); os.IsNotExist(err) {
		return nil, err
	}
	if err := nc.osSetenv(goPSUtilProcDirEnv, "/rootfs/proc"); err != nil {
		return nil, fmt.Errorf("NodeCapacity cannot set goPSUtilProcDirEnv to /rootfs/proc: %v", err)
	}

	nc.parseCPU()
	nc.parseMemory()
	return nc, nil
}

func (nc *NodeCapacity) parseMemory() {
	if memStats, err := nc.virtualMemory(); err == nil {
		nc.MemCapacity = int64(memStats.Total)
	} else {
		// If any error happen, then there will be no mem utilization metrics
		nc.logger.Error("NodeCapacity cannot get memStats from psUtil", zap.Error(err))
	}
}

func (nc *NodeCapacity) parseCPU() {
	if cpuInfos, err := nc.cpuInfo(); err == nil {
		numCores := len(cpuInfos)
		nc.CPUCapacity = int64(numCores)
	} else {
		// If any error happen, then there will be no cpu utilization metrics
		nc.logger.Error("NodeCapacity cannot get cpuInfo from psUtil", zap.Error(err))
	}
}

func (nc *NodeCapacity) GetNumCores() int64 {
	return nc.CPUCapacity
}

func (nc *NodeCapacity) GetMemoryCapacity() int64 {
	return nc.MemCapacity
}
