// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package commander

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

const passthroughTestModeEnv = "OTEL_SUPERVISOR_COMMANDER_TEST_MODE" // #nosec G101 -- Test mode selector, not a credential.

func TestMain(m *testing.M) {
	if os.Getenv(passthroughTestModeEnv) == "passthrough" {
		// Re-run this test binary as the child process so the test can assert
		// Commander drains passthrough logs before reporting process exit.
		_, _ = fmt.Fprint(os.Stderr, "final error line")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestPassthroughLogsDrainedBeforeExited(t *testing.T) {
	cmdr, err := NewCommander(
		zap.NewNop(),
		t.TempDir(),
		config.Agent{
			Executable:      os.Args[0],
			PassthroughLogs: true,
			Env: map[string]string{
				passthroughTestModeEnv: "passthrough",
			},
		},
	)
	require.NoError(t, err)

	var mu sync.Mutex
	var lines []string
	cmdr.SetPassthroughLogHook(func(line string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, line)
	})

	require.NoError(t, cmdr.Start(t.Context()))

	select {
	case <-cmdr.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for commander exit")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"final error line"}, lines)
}
