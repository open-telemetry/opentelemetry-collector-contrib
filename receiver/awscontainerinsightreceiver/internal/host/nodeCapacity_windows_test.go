// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package host

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNodeCapacityOnWindows(t *testing.T) {
	// can't parse cpu and mem info
	setEnvOption := func(nc *nodeCapacity) {
		nc.osSetenv = func(key, value string) error {
			return nil
		}
	}
	virtualMemOption := func(nc *nodeCapacity) {
		nc.virtualMemory = func(ctx context.Context) (*mem.VirtualMemoryStat, error) {
			return nil, errors.New("error")
		}
	}
	cpuInfoOption := func(nc *nodeCapacity) {
		nc.cpuInfo = func(ctx context.Context) ([]cpu.InfoStat, error) {
			return nil, errors.New("error")
		}
	}
	nc, err := newNodeCapacity(zap.NewNop(), setEnvOption, virtualMemOption, cpuInfoOption)
	assert.NotNil(t, nc)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), nc.getMemoryCapacity())
	assert.Equal(t, int64(0), nc.getNumCores())

	// normal case where everything is working
	virtualMemOption = func(nc *nodeCapacity) {
		nc.virtualMemory = func(ctx context.Context) (*mem.VirtualMemoryStat, error) {
			return &mem.VirtualMemoryStat{
				Total: 1024,
			}, nil
		}
	}
	cpuInfoOption = func(nc *nodeCapacity) {
		nc.cpuInfo = func(ctx context.Context) ([]cpu.InfoStat, error) {
			return []cpu.InfoStat{
				{Cores: 1},
				{Cores: 1},
			}, nil
		}
	}
	nc, err = newNodeCapacity(zap.NewNop(), setEnvOption, virtualMemOption, cpuInfoOption)
	assert.NotNil(t, nc)
	assert.Nil(t, err)
	assert.Equal(t, int64(1024), nc.getMemoryCapacity())
	assert.Equal(t, int64(2), nc.getNumCores())
}
