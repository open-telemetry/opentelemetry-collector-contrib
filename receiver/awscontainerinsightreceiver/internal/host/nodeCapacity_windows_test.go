// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package host

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNodeCapacityOnWindows(t *testing.T) {
	// can't parse cpu and mem info
	virtualMemOption := func(nc any) {
		nc.(*nodeCapacity).virtualMemory = func(_ context.Context) (*mem.VirtualMemoryStat, error) {
			return nil, errors.New("error")
		}
	}
	cpuInfoOption := func(nc any) {
		nc.(*nodeCapacity).cpuInfo = func(_ context.Context) ([]cpu.InfoStat, error) {
			return nil, errors.New("error")
		}
	}
	nc, err := newNodeCapacity(zap.NewNop(), virtualMemOption, cpuInfoOption)
	assert.NotNil(t, nc)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), nc.getMemoryCapacity())
	assert.Equal(t, int64(0), nc.getNumCores())

	// normal case where everything is working
	virtualMemOption = func(nc any) {
		nc.(*nodeCapacity).virtualMemory = func(_ context.Context) (*mem.VirtualMemoryStat, error) {
			return &mem.VirtualMemoryStat{
				Total: 1024,
			}, nil
		}
	}
	cpuInfoOption = func(nc any) {
		nc.(*nodeCapacity).cpuInfo = func(_ context.Context) ([]cpu.InfoStat, error) {
			return []cpu.InfoStat{
				{Cores: 1},
				{Cores: 1},
			}, nil
		}
	}
	nc, err = newNodeCapacity(zap.NewNop(), virtualMemOption, cpuInfoOption)
	assert.NotNil(t, nc)
	assert.NoError(t, err)
	assert.Equal(t, int64(1024), nc.getMemoryCapacity())
	assert.Equal(t, int64(2), nc.getNumCores())
}
