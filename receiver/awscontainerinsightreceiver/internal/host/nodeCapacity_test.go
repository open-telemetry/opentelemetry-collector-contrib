// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNodeCapacity(t *testing.T) {
	// no proc directory
	lstatOption := func(nc *nodeCapacity) {
		nc.osLstat = func(name string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		}
	}
	nc, err := newNodeCapacity(zap.NewNop(), lstatOption)
	assert.Nil(t, nc)
	assert.NotNil(t, err)

	// can't set environment variables
	lstatOption = func(nc *nodeCapacity) {
		nc.osLstat = func(name string) (os.FileInfo, error) {
			return nil, nil
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
	nc, err = newNodeCapacity(zap.NewNop(), lstatOption, virtualMemOption, cpuInfoOption)
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
				{},
				{},
			}, nil
		}
	}
	nc, err = newNodeCapacity(zap.NewNop(), lstatOption, virtualMemOption, cpuInfoOption)
	assert.NotNil(t, nc)
	assert.Nil(t, err)
	assert.Equal(t, int64(1024), nc.getMemoryCapacity())
	assert.Equal(t, int64(2), nc.getNumCores())
}
