// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"fmt"
	"os"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"
)

const (
	goPSUtilProcDirEnv = "HOST_PROC"
)

type nodeCapacityProvider interface {
	getMemoryCapacity() int64
	getNumCores() int64
}

type nodeCapacity struct {
	memCapacity int64
	cpuCapacity int64
	logger      *zap.Logger

	// osLstat returns a FileInfo describing the named file.
	osLstat func(name string) (os.FileInfo, error)
	// osSetenv sets the value of the environment variable named by the key
	osSetenv      func(key string, value string) error
	virtualMemory func() (*mem.VirtualMemoryStat, error)
	cpuInfo       func() ([]cpu.InfoStat, error)
}

type nodeCapacityOption func(*nodeCapacity)

func newNodeCapacity(logger *zap.Logger, options ...nodeCapacityOption) (nodeCapacityProvider, error) {
	nc := &nodeCapacity{
		logger:        logger,
		osLstat:       os.Lstat,
		osSetenv:      os.Setenv,
		virtualMemory: mem.VirtualMemory,
		cpuInfo:       cpu.Info,
	}

	for _, opt := range options {
		opt(nc)
	}

	if _, err := nc.osLstat(hostProc); os.IsNotExist(err) {
		return nil, err
	}
	if err := nc.osSetenv(goPSUtilProcDirEnv, hostProc); err != nil {
		return nil, fmt.Errorf("NodeCapacity cannot set goPSUtilProcDirEnv to %s: %w", hostProc, err)
	}

	nc.parseCPU()
	nc.parseMemory()
	return nc, nil
}

func (nc *nodeCapacity) parseMemory() {
	if memStats, err := nc.virtualMemory(); err == nil {
		nc.memCapacity = int64(memStats.Total)
	} else {
		// If any error happen, then there will be no mem utilization metrics
		nc.logger.Error("NodeCapacity cannot get memStats from psUtil", zap.Error(err))
	}
}

func (nc *nodeCapacity) parseCPU() {
	if cpuInfos, err := nc.cpuInfo(); err == nil {
		numCores := len(cpuInfos)
		nc.cpuCapacity = int64(numCores)
	} else {
		// If any error happen, then there will be no cpu utilization metrics
		nc.logger.Error("NodeCapacity cannot get cpuInfo from psUtil", zap.Error(err))
	}
}

func (nc *nodeCapacity) getNumCores() int64 {
	return nc.cpuCapacity
}

func (nc *nodeCapacity) getMemoryCapacity() int64 {
	return nc.memCapacity
}
