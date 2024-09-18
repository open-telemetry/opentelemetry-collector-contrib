// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"testing"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

type CancelFunc = func() error

func joinCgroup(pid uint64) (CancelFunc, error) {
	var max int64 = 100000
	control, err := cgroup2.NewManager("/tmp/otel-col.extension.slice", "/", &cgroup2.Resources{
		Memory: &cgroup2.Memory{
			Max: &max,
		},
	})
	if err != nil {
		return nil, err
	}
	return control.Delete, control.AddProc(pid)
}

func TestExtension(t *testing.T) {
	// Only test in Linux environments with cgroup available
	if cgroups.Mode() == cgroups.Unavailable {
		t.Skip()
	}

	tests := []struct {
		name               string
		config             *Config
		expectedGoMaxProcs int
		expectedGoMemLimit float64
	}{
		{
			name: "mem limit",
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enable: false,
				},
				GoMemLimit: GoMemLimitConfig{
					Enable: true,
					Ratio:  0.5,
				},
			},
			expectedGoMemLimit: 0.5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shutdown, err := joinCgroup(uint64(os.Getpid()))
			if err != nil {
				t.Fatal(err)
			}
			defer shutdown()

			initialLimit := debug.SetMemoryLimit(-1)

			cg := newCgroupRuntime(test.config, componenttest.NewNopTelemetrySettings())
			ctx := context.Background()
			err = cg.Start(ctx, componenttest.NewNopHost())
			require.NoError(t, err)

			// Using a negative value will just return the actual one.
			// require.Equal(t, 90, runtime.GOMAXPROCS(-1))
			actualLimit := debug.SetMemoryLimit(-1)
			fmt.Printf("Initial: %v Actual: %v", initialLimit, actualLimit)
			require.Equal(t, test.expectedGoMemLimit, actualLimit/initialLimit)

			require.NoError(t, cg.Shutdown(ctx))
		})
	}
}
