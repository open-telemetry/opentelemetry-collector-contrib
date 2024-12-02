// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build integration
// +build integration

package cgroupruntimeextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"

	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

const (
	defaultCgroup2Path = "/sys/fs/cgroup"
)

// cgroupMaxCpu returns the CPU max definition for a given cgroup slice path
// File format: cpu_quote cpu_period
func cgroupMaxCpu(filename string) (quota int64, period uint64, err error) {
	out, err := os.ReadFile(filepath.Join(defaultCgroup2Path, filename, "cpu.max"))
	if err != nil {
		return 0, 0, err
	}
	values := strings.Split(strings.TrimSpace(string(out)), " ")
	if values[0] == "max" {
		quota = math.MaxInt64
	} else {
		quota, _ = strconv.ParseInt(values[0], 10, 64)
	}
	period, _ = strconv.ParseUint(values[1], 10, 64)
	return quota, period, err
}

func TestCgroupV2Integration(t *testing.T) {
	pointerInt64 := func(int int64) *int64 {
		return &int
	}
	pointerUint64 := func(int uint64) *uint64 {
		return &int
	}

	tests := []struct {
		name string
		// nil CPU quota == "max" cgroup string value
		cgroupCpuQuota  *int64
		cgroupCpuPeriod uint64
		config          *Config
	}{
		{
			name:            "90% the max cgroup memory and 12 GOMAXPROCS",
			cgroupCpuQuota:  pointerInt64(100000),
			cgroupCpuPeriod: 8000,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.9,
				},
			},
		},
		{
			name:            "0.5 of the max cgroup memory and 1 GOMAXPROCS",
			cgroupCpuQuota:  pointerInt64(100000),
			cgroupCpuPeriod: 100000,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.5,
				},
			},
		},
		{
			name:            "default GOMAXPROCS",
			cgroupCpuQuota:  nil,
			cgroupCpuPeriod: 100000,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.1,
				},
			},
		},
	}

	cgroupPath, err := cgroup2.PidGroupPath(os.Getpid())
	assert.NoError(t, err)
	manager, err := cgroup2.Load(cgroupPath)
	assert.NoError(t, err)

	stats, err := manager.Stat()
	require.NoError(t, err)

	// 128 MB
	initialBaseMaxMemory := 134217728
	initialMaxMemory := stats.GetMemory().GetUsageLimit()
	initialCpuQuota, initialCpuPeriod, err := cgroupMaxCpu(cgroupPath)
	require.NoError(t, err)

	fmt.Printf("Initial max memory: %v\n", initialMaxMemory)
	previous := debug.SetMemoryLimit(math.MaxInt64)
	fmt.Printf("Previous test: %v\n", previous)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initialGoMem := debug.SetMemoryLimit(-1)
			initialGoProcs := runtime.GOMAXPROCS(-1)

			// restore startup cgroup initial resource values
			t.Cleanup(func() {
				debug.SetMemoryLimit(initialGoMem)
				runtime.GOMAXPROCS(initialGoProcs)
				err = manager.Update(&cgroup2.Resources{
					Memory: &cgroup2.Memory{
						Max: pointerInt64(int64(initialMaxMemory)),
					},
					CPU: &cgroup2.CPU{
						Max: cgroup2.NewCPUMax(pointerInt64(initialCpuQuota), pointerUint64(initialCpuPeriod)),
					},
				})
			})

			err = manager.Update(&cgroup2.Resources{
				Memory: &cgroup2.Memory{
					// Default max memory must be
					// overwritten
					// to automemlimit change the GOMEMLIMIT
					// value
					Max: pointerInt64(int64(initialBaseMaxMemory)),
				},
				CPU: &cgroup2.CPU{
					Max: cgroup2.NewCPUMax(test.cgroupCpuQuota, pointerUint64(test.cgroupCpuPeriod)),
				},
			})
			assert.NoError(t, err)

			stats, err := manager.Stat()
			require.NoError(t, err)

			maxMemory := stats.Memory.UsageLimit
			fmt.Printf("After test: %v\n", maxMemory)

			fmt.Printf("GOMEMLIMIT before: %v\n", debug.SetMemoryLimit(-1))
			factory := NewFactory()
			ctx := context.Background()
			extension, err := factory.Create(ctx, extensiontest.NewNopSettings(), test.config)
			assert.NoError(t, err)

			err = extension.Start(ctx, componenttest.NewNopHost())
			assert.NoError(t, err)
			fmt.Printf("GOMEMLIMIT after: %v\n", debug.SetMemoryLimit(-1))

			assert.Equal(t, uint64(float64(initialBaseMaxMemory)*test.config.GoMemLimit.Ratio), uint64(debug.SetMemoryLimit(-1)))
			// GOMAXPROCS is set to the value of  `cpu.max / cpu.period`
			// If cpu.max is set to max, GOMAXPROCS should not be
			// modified
			if test.cgroupCpuQuota == nil {
				assert.Equal(t, initialGoProcs, runtime.GOMAXPROCS(-1))
			} else {
				assert.Equal(t, *test.cgroupCpuQuota/int64(test.cgroupCpuPeriod), int64(runtime.GOMAXPROCS(-1)))
			}
		})
	}
}
