// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host

import (
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
	setEnvOption := func(nc *nodeCapacity) {
		nc.osSetenv = func(key, value string) error {
			return errors.New("error")
		}
	}
	nc, err = newNodeCapacity(zap.NewNop(), lstatOption, setEnvOption)
	assert.Nil(t, nc)
	assert.NotNil(t, err)

	// can't parse cpu and mem info
	setEnvOption = func(nc *nodeCapacity) {
		nc.osSetenv = func(key, value string) error {
			return nil
		}
	}
	virtualMemOption := func(nc *nodeCapacity) {
		nc.virtualMemory = func() (*mem.VirtualMemoryStat, error) {
			return nil, errors.New("error")
		}
	}
	cpuInfoOption := func(nc *nodeCapacity) {
		nc.cpuInfo = func() ([]cpu.InfoStat, error) {
			return nil, errors.New("error")
		}
	}
	nc, err = newNodeCapacity(zap.NewNop(), lstatOption, setEnvOption, virtualMemOption, cpuInfoOption)
	assert.NotNil(t, nc)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), nc.getMemoryCapacity())
	assert.Equal(t, int64(0), nc.getNumCores())

	// normal case where everything is working
	virtualMemOption = func(nc *nodeCapacity) {
		nc.virtualMemory = func() (*mem.VirtualMemoryStat, error) {
			return &mem.VirtualMemoryStat{
				Total: 1024,
			}, nil
		}
	}
	cpuInfoOption = func(nc *nodeCapacity) {
		nc.cpuInfo = func() ([]cpu.InfoStat, error) {
			return []cpu.InfoStat{
				{},
				{},
			}, nil
		}
	}
	nc, err = newNodeCapacity(zap.NewNop(), lstatOption, setEnvOption, virtualMemOption, cpuInfoOption)
	assert.NotNil(t, nc)
	assert.Nil(t, err)
	assert.Equal(t, int64(1024), nc.getMemoryCapacity())
	assert.Equal(t, int64(2), nc.getNumCores())
}
