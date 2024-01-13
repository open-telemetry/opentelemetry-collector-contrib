// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"context"
	"os"

	"github.com/shirou/gopsutil/v3/common"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"
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
	osLstat       func(name string) (os.FileInfo, error)
	virtualMemory func(ctx context.Context) (*mem.VirtualMemoryStat, error)
	cpuInfo       func(ctx context.Context) ([]cpu.InfoStat, error)
}

type nodeCapacityOption func(*nodeCapacity)

func newNodeCapacity(logger *zap.Logger, options ...nodeCapacityOption) (nodeCapacityProvider, error) {
	nc := &nodeCapacity{
		logger:        logger,
		osLstat:       os.Lstat,
		virtualMemory: mem.VirtualMemoryWithContext,
		cpuInfo:       cpu.InfoWithContext,
	}

	for _, opt := range options {
		opt(nc)
	}

	if _, err := nc.osLstat(hostProc); os.IsNotExist(err) {
		return nil, err
	}
	envMap := common.EnvMap{common.HostProcEnvKey: hostProc}
	ctx := context.WithValue(context.Background(), common.EnvKey, envMap)

	nc.parseCPU(ctx)
	nc.parseMemory(ctx)
	return nc, nil
}

func (nc *nodeCapacity) parseMemory(ctx context.Context) {
	if memStats, err := nc.virtualMemory(ctx); err == nil {
		nc.memCapacity = int64(memStats.Total)
	} else {
		// If any error happen, then there will be no mem utilization metrics
		nc.logger.Error("NodeCapacity cannot get memStats from psUtil", zap.Error(err))
	}
}

func (nc *nodeCapacity) parseCPU(ctx context.Context) {
	if cpuInfos, err := nc.cpuInfo(ctx); err == nil {
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
