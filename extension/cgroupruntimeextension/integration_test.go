// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build integration && linux
// +build integration,linux

// Privileged access is required to set cgroup's memory and cpu max values

package cgroupruntimeextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
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
	"golang.org/x/sys/unix"
)

const (
	defaultCgroup2Path = "/sys/fs/cgroup"
	ecsMetadataURI     = "ECS_CONTAINER_METADATA_URI_V4"
)

// checkCgroupSystem skips the test if is not run in a cgroupv2 system
func checkCgroupSystem(tb testing.TB) {
	var st unix.Statfs_t
	err := unix.Statfs(defaultCgroup2Path, &st)
	if err != nil {
		tb.Skip("cannot statfs cgroup root")
	}

	isUnified := st.Type == unix.CGROUP2_SUPER_MAGIC
	if !isUnified {
		tb.Skip("System running in hybrid or cgroupv1 mode")
	}
}

func pointerInt64(val int64) *int64 {
	return &val
}

func pointerUint64(uval uint64) *uint64 {
	return &uval
}

// setupMemoryCgroupCleanUp returns a cleanup function that restores the cgroup's max memory to its initial value
func setupMemoryCgroupCleanUp(t *testing.T, manager *cgroup2.Manager, cgroupPath string) func() {
	stats, err := manager.Stat()
	require.NoError(t, err)

	initialMaxMemory := stats.GetMemory().GetUsageLimit()
	memoryCgroupCleanUp := func() {
		err = manager.Update(&cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Max: pointerInt64(int64(initialMaxMemory)),
			},
		})
		assert.NoError(t, err)
	}

	if initialMaxMemory == math.MaxUint64 {
		// fallback solution to set cgroup's max memory to "max"
		memoryCgroupCleanUp = func() {
			err = os.WriteFile(path.Join(defaultCgroup2Path, cgroupPath, "memory.max"), []byte("max"), 0o600)
			assert.NoError(t, err)
		}
	}
	return memoryCgroupCleanUp
}

// cgroupMaxCpu returns the CPU max definition for a given cgroup slice path
// File format: cpu_quote cpu_period
func cgroupMaxCPU(filename string) (quota int64, period uint64, err error) {
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

// startExtension starts the extension with the given config
func startExtension(t *testing.T, config *Config) {
	factory := NewFactory()
	ctx := context.Background()
	extension, err := factory.Create(ctx, extensiontest.NewNopSettings(), config)
	require.NoError(t, err)

	err = extension.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
}

func TestCgroupV2SudoIntegration(t *testing.T) {
	checkCgroupSystem(t)

	tests := []struct {
		name string
		// nil CPU quota == "max" cgroup string value
		cgroupCPUQuota     *int64
		cgroupCPUPeriod    uint64
		cgroupMaxMemory    int64
		config             *Config
		expectedGoMaxProcs int
		expectedGoMemLimit int64
	}{
		{
			name:            "90% the max cgroup memory and 12 GOMAXPROCS",
			cgroupCPUQuota:  pointerInt64(100000),
			cgroupCPUPeriod: 8000,
			// 128 Mb
			cgroupMaxMemory: 134217728,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.9,
				},
			},
			// 100000 / 8000
			expectedGoMaxProcs: 12,
			// 134217728 * 0.9
			expectedGoMemLimit: 120795955,
		},
		{
			name:            "50% of the max cgroup memory and 1 GOMAXPROCS",
			cgroupCPUQuota:  pointerInt64(100000),
			cgroupCPUPeriod: 100000,
			// 128 Mb
			cgroupMaxMemory: 134217728,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.5,
				},
			},
			// 100000 / 100000
			expectedGoMaxProcs: 1,
			// 134217728 * 0.5
			expectedGoMemLimit: 67108864,
		},
		{
			name:            "10% of the max cgroup memory, max cpu, default GOMAXPROCS",
			cgroupCPUQuota:  nil,
			cgroupCPUPeriod: 100000,
			// 128 Mb
			cgroupMaxMemory: 134217728,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.1,
				},
			},
			// GOMAXPROCS is set to the value of  `cpu.max / cpu.period`
			// If cpu.max is set to max, GOMAXPROCS should not be
			// modified
			expectedGoMaxProcs: runtime.GOMAXPROCS(-1),
			// 134217728 * 0.1
			expectedGoMemLimit: 13421772,
		},
	}

	cgroupPath, err := cgroup2.PidGroupPath(os.Getpid())
	assert.NoError(t, err)
	manager, err := cgroup2.Load(cgroupPath)
	assert.NoError(t, err)

	// Startup resource values
	memoryCgroupCleanUp := setupMemoryCgroupCleanUp(t, manager, cgroupPath)

	initialCPUQuota, initialCPUPeriod, err := cgroupMaxCPU(cgroupPath)
	require.NoError(t, err)
	cpuCgroupCleanUp := func() {
		fmt.Println(initialCPUQuota)
		err = manager.Update(&cgroup2.Resources{
			CPU: &cgroup2.CPU{
				Max: cgroup2.NewCPUMax(pointerInt64(initialCPUQuota), pointerUint64(initialCPUPeriod)),
			},
		})
		assert.NoError(t, err)
	}

	if initialCPUQuota == math.MaxInt64 {
		// fallback solution to set cgroup's max cpu to "max"
		cpuCgroupCleanUp = func() {
			err = os.WriteFile(path.Join(defaultCgroup2Path, cgroupPath, "cpu.max"), []byte("max"), 0o600)
			assert.NoError(t, err)
		}
	}

	initialGoMem := debug.SetMemoryLimit(-1)
	initialGoProcs := runtime.GOMAXPROCS(-1)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// restore startup cgroup initial resource values
			t.Cleanup(func() {
				debug.SetMemoryLimit(initialGoMem)
				runtime.GOMAXPROCS(initialGoProcs)
				memoryCgroupCleanUp()
				cpuCgroupCleanUp()
			})

			err = manager.Update(&cgroup2.Resources{
				Memory: &cgroup2.Memory{
					// Default max memory must be
					// overwritten
					// to automemlimit change the GOMEMLIMIT
					// value
					Max: pointerInt64(test.cgroupMaxMemory),
				},
				CPU: &cgroup2.CPU{
					Max: cgroup2.NewCPUMax(test.cgroupCPUQuota, pointerUint64(test.cgroupCPUPeriod)),
				},
			})
			require.NoError(t, err)

			startExtension(t, test.config)

			assert.Equal(t, test.expectedGoMaxProcs, runtime.GOMAXPROCS(-1))
			assert.Equal(t, test.expectedGoMemLimit, debug.SetMemoryLimit(-1))
		})
	}
}

func testServerECSMetadata(t *testing.T, containerCPU, taskCPU int) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(fmt.Sprintf(`{"Limits":{"CPU":%d},"DockerId":"container-id"}`, containerCPU)))
		assert.NoError(t, err)
	})
	mux.HandleFunc("/task", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(fmt.Sprintf(
			`{"Containers":[{"DockerId":"container-id","Limits":{"CPU":%d}}],"Limits":{"CPU":%d}}`,
			containerCPU,
			taskCPU,
		)))
		assert.NoError(t, err)
	})

	return httptest.NewServer(mux)
}

func TestECSCgroupV2SudoIntegration(t *testing.T) {
	checkCgroupSystem(t)

	tests := []struct {
		name               string
		containerCPU       int
		taskCPU            int
		cgroupMaxMemory    int64
		config             *Config
		expectedGoMaxProcs int
		expectedGoMemLimit int64
	}{
		{
			name:         "90% the max cgroup memory and 4 GOMAXPROCS w/ 4096 container cpu 16 task cpu",
			containerCPU: 4096,
			taskCPU:      16,
			// 128 Mb
			cgroupMaxMemory: 134217728,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.9,
				},
			},
			expectedGoMaxProcs: 4,
			// 134217728 * 0.9
			expectedGoMemLimit: 120795955,
		},
		{
			name:         "50% of the max cgroup memory and 1 GOMAXPROCS w/ 2048 container cpu 2 task cpu",
			containerCPU: 2048,
			taskCPU:      2,
			// 128 Mb
			cgroupMaxMemory: 134217728,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.5,
				},
			},
			expectedGoMaxProcs: 2,
			// 134217728 * 0.5
			expectedGoMemLimit: 67108864,
		},
		{
			name:         "50% of the max cgroup memory and 1 GOMAXPROCS w/ 1024 container cpu 4 task cpu",
			containerCPU: 1024,
			taskCPU:      4,
			// 128 Mb
			cgroupMaxMemory: 134217728,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.5,
				},
			},
			expectedGoMaxProcs: 1,
			// 134217728 * 0.5
			expectedGoMemLimit: 67108864,
		},
		{
			name:         "10% of the max cgroup memory and 4 GOMAXPROCS w/ 4096 container cpu 0 task cpu",
			containerCPU: 4096,
			taskCPU:      0,
			// 128 Mb
			cgroupMaxMemory: 134217728,
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.1,
				},
			},
			expectedGoMaxProcs: 4,
			// 134217728 * 0.1
			expectedGoMemLimit: 13421772,
		},
	}

	cgroupPath, err := cgroup2.PidGroupPath(os.Getpid())
	assert.NoError(t, err)
	manager, err := cgroup2.Load(cgroupPath)
	assert.NoError(t, err)

	// Startup resource values
	memoryCgroupCleanUp := setupMemoryCgroupCleanUp(t, manager, cgroupPath)

	initialGoMem := debug.SetMemoryLimit(-1)
	initialGoProcs := runtime.GOMAXPROCS(-1)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// running in ECS environment, set the ECS metedata URI environment variable
			// to get the Cgroup CPU quota from the httptest server
			server := testServerECSMetadata(t, test.containerCPU, test.taskCPU)
			t.Setenv(ecsMetadataURI, server.URL)
			// restore startup cgroup initial resource values
			t.Cleanup(func() {
				debug.SetMemoryLimit(initialGoMem)
				runtime.GOMAXPROCS(initialGoProcs)
				memoryCgroupCleanUp()
				server.Close()
				os.Unsetenv(ecsMetadataURI)
			})

			err = manager.Update(&cgroup2.Resources{
				Memory: &cgroup2.Memory{
					// Default max memory must be
					// overwritten
					// to automemlimit change the GOMEMLIMIT
					// value
					Max: pointerInt64(test.cgroupMaxMemory),
				},
			})
			require.NoError(t, err)

			startExtension(t, test.config)

			assert.Equal(t, test.expectedGoMaxProcs, runtime.GOMAXPROCS(-1))
			assert.Equal(t, test.expectedGoMemLimit, debug.SetMemoryLimit(-1))
		})
	}
}
