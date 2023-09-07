// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subprocess

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSubprocessAndConfig(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{}
	subprocess := NewSubprocess(config, logger)
	require.NotNil(t, subprocess)
	require.Same(t, config, subprocess.config)
	require.Same(t, logger, subprocess.logger)
	require.NotNil(t, subprocess.Stdout)

	require.Equal(t, *config.ShutdownTimeout, 5*time.Second)
	require.Equal(t, *config.RestartDelay, 5*time.Second)
}

func TestConfigDurations(t *testing.T) {
	logger := zap.NewNop()
	restartDelay := 100 * time.Second
	shutdownTimeout := 200 * time.Second
	config := &Config{RestartDelay: &restartDelay, ShutdownTimeout: &shutdownTimeout}
	subprocess := NewSubprocess(config, logger)
	require.NotNil(t, subprocess)
	require.Equal(t, *config.ShutdownTimeout, shutdownTimeout)
	require.Equal(t, *config.RestartDelay, restartDelay)
}

func TestShutdownTimeout(t *testing.T) {
	logger := zap.NewNop()
	timeout := 10 * time.Millisecond
	config := &Config{ShutdownTimeout: &timeout}
	subprocess := NewSubprocess(config, logger)
	require.NotNil(t, subprocess)

	_, cancel := context.WithCancel(context.Background())
	subprocess.cancel = cancel

	t0 := time.Now()
	err := subprocess.Shutdown(context.Background())
	require.NoError(t, err)

	elapsed := int64(time.Since(t0))
	require.GreaterOrEqual(t, elapsed, int64(10*time.Millisecond))
	require.GreaterOrEqual(t, int64(1*time.Second), elapsed)
}

func TestPidAccessors(t *testing.T) {
	subprocess := &Subprocess{}
	require.Equal(t, -1, subprocess.Pid())

	subprocess = NewSubprocess(&Config{}, nil)
	require.Equal(t, -1, subprocess.Pid())

	subprocess.pid.setPid(123)
	require.Equal(t, 123, subprocess.pid.getPid())
	require.Equal(t, 123, subprocess.Pid())

}
