// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration && !windows

package subprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const scriptContents = `#!/bin/sh
echo "Env:" $(env)
echo "Stdin:" $(cat -)

# this will be canceled and shouldn't influence overall test time
sleep 60
`

func setupScriptPath(t *testing.T) string {
	scriptFile, err := os.CreateTemp(t.TempDir(), "subproc")
	require.NoError(t, err)

	_, err = scriptFile.WriteString(scriptContents)
	require.NoError(t, err)
	require.NoError(t, scriptFile.Chmod(0o700))
	scriptFile.Close()

	return scriptFile.Name()
}

func cleanupScriptPath(t *testing.T, scriptPath string) {
	require.NoError(t, os.Remove(scriptPath))
}

// prepareSubprocess will create a Subprocess based on a temporary script.
// It returns a pointer to the pointer to psutil process info and a closure to set its
// value from the running process once started.
func prepareSubprocess(conf *Config) (*Subprocess, func(t *assert.CollectT) *process.Process) {
	logCore, _ := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	subprocess := NewSubprocess(conf, logger)

	selfPid := int32(os.Getpid())
	expectedExecutable := fmt.Sprintf("/bin/sh %v", conf.ExecutablePath)

	findProcessInfo := func(t *assert.CollectT) *process.Process {
		pid := int32(subprocess.Pid())
		assert.NotEqual(t, pid, -1)
		proc, err := process.NewProcess(pid)
		require.NoError(t, err)
		ppid, err := proc.Ppid()
		require.NoError(t, err)
		require.Equal(t, selfPid, ppid)

		cmdline, err := proc.Cmdline()
		assert.NotEmpty(t, cmdline)
		require.NoError(t, err)
		require.Truef(t, strings.HasPrefix(cmdline, expectedExecutable), "%v doesn't have prefix %v", cmdline, expectedExecutable)
		return proc
	}

	return subprocess, findProcessInfo
}

func requireDesiredStdout(t *testing.T, subprocess *Subprocess, desired string) {
	timer := time.After(5 * time.Second)
loop:
	for {
		select {
		case <-timer:
			break loop
		case out := <-subprocess.Stdout:
			if out == desired {
				return
			}
		}
	}
	t.Fatal("Failed to receive desired stdout")
}

func TestHappyPath(t *testing.T) {
	scriptPath := setupScriptPath(t)
	t.Cleanup(func() {
		cleanupScriptPath(t, scriptPath)
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	subprocess, findProcessInfo := prepareSubprocess(&Config{
		ExecutablePath: scriptPath,
	})
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	assert.EventuallyWithT(t, func(collect *assert.CollectT) {
		procInfo := findProcessInfo(collect)
		require.NotNil(collect, procInfo)
		cmdline, err := (procInfo).Cmdline()
		assert.NoError(t, err)
		assert.Equal(collect, "/bin/sh "+subprocess.config.ExecutablePath, cmdline)
	}, 5*time.Second, 10*time.Millisecond)
}

func TestWithArgs(t *testing.T) {
	scriptPath := setupScriptPath(t)
	t.Cleanup(func() {
		cleanupScriptPath(t, scriptPath)
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	subprocess, findProcessInfo := prepareSubprocess(&Config{
		ExecutablePath: scriptPath,
		Args:           []string{"myArgs"},
	})
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		procInfo := findProcessInfo(collect)
		require.NotNil(collect, procInfo)
		cmdline, err := (procInfo).Cmdline()
		require.NoError(collect, err)
		require.Equal(collect, fmt.Sprintf("/bin/sh %v myArgs", subprocess.config.ExecutablePath), cmdline)
	}, 5*time.Second, 10*time.Millisecond)
}

func TestWithEnvVars(t *testing.T) {
	scriptPath := setupScriptPath(t)
	t.Cleanup(func() {
		cleanupScriptPath(t, scriptPath)
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	config := &Config{
		ExecutablePath: scriptPath,
		EnvironmentVariables: map[string]string{
			"MyEnv1": "MyVal1",
			"MyEnv2": "MyVal2",
		},
	}

	subprocess, findProcessInfo := prepareSubprocess(config)
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		require.NotNil(collect, findProcessInfo(collect))
	}, 5*time.Second, 10*time.Millisecond)

	stdout := <-subprocess.Stdout
	require.NotEmpty(t, stdout)
	require.Contains(t, stdout, "MyEnv1=MyVal1")
	require.Contains(t, stdout, "MyEnv2=MyVal2")
}

func TestWithAutoRestart(t *testing.T) {
	scriptPath := setupScriptPath(t)
	t.Cleanup(func() {
		cleanupScriptPath(t, scriptPath)
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	restartDelay := 100 * time.Millisecond
	subprocess, findProcessInfo := prepareSubprocess(&Config{
		ExecutablePath: scriptPath,
		RestartOnError: true, RestartDelay: &restartDelay,
	})
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	var oldProcPid int32
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		procInfo := findProcessInfo(collect)
		require.NotNil(collect, procInfo)
		cmdline, err := (*procInfo).Cmdline()
		require.NoError(collect, err)
		require.Equal(collect, "/bin/sh "+subprocess.config.ExecutablePath, cmdline)

		oldProcPid = (procInfo).Pid
		require.NoError(collect, (procInfo).Kill())
	}, 5*time.Second, 10*time.Millisecond)

	// Should be restarted
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		procInfo := findProcessInfo(collect)
		assert.True(collect, procInfo != nil && (procInfo).Pid != oldProcPid)
	}, restartDelay+5*time.Second, 10*time.Millisecond)
}

func TestSendingStdin(t *testing.T) {
	scriptPath := setupScriptPath(t)
	t.Cleanup(func() {
		cleanupScriptPath(t, scriptPath)
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	subprocess, findProcessInfo := prepareSubprocess(&Config{
		ExecutablePath: scriptPath,
		StdInContents:  "mystdincontents",
	})
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		require.NotNil(collect, findProcessInfo(collect))
	}, 5*time.Second, 10*time.Millisecond)

	requireDesiredStdout(t, subprocess, "Stdin: mystdincontents")
}

func TestSendingStdinFails(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	logCore, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	subprocess := NewSubprocess(&Config{
		ExecutablePath: "echo", Args: []string{"finished"},
	}, logger)

	intentionalError := errors.New("intentional failure")
	subprocess.sendToStdIn = func(string, io.Writer) error {
		return intentionalError
	}

	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	matched := func() bool {
		died := len(logObserver.FilterMessage("subprocess died").All()) == 1
		errored := len(logObserver.FilterField(zap.Error(intentionalError)).All()) == 1
		return died && errored
	}

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.True(collect, matched())
	}, 10*time.Second, 10*time.Millisecond)
}

func TestSubprocessBadExec(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	logCore, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	subprocess := NewSubprocess(&Config{ExecutablePath: "/does/not/exist"}, logger)
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	matched := func() bool {
		return len(logObserver.FilterMessage("subprocess died").All()) == 1
	}

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.True(collect, matched())
	}, 10*time.Second, 10*time.Millisecond)
}

func TestSubprocessSuccessfullyReturns(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// There is a race condition between writing from the stdout scanner and the closing of the stdout channel on
	// process exit. Here we sleep before returning, but this will need to be addressed if short lived processes
	// become an intended use case without forcing users to read stdout before closing.
	subprocess := NewSubprocess(&Config{ExecutablePath: "sh", Args: []string{"-c", "echo finished; sleep .1"}}, zap.NewNop())
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	matched := func() bool {
		_, ok := <-subprocess.shutdownSignal
		return !ok
	}

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.True(collect, matched())
	}, 10*time.Second, 10*time.Millisecond)
	requireDesiredStdout(t, subprocess, "finished")
}

func TestShutdownBeforeStartIntegration(t *testing.T) {
	t.Parallel()
	subprocess := NewSubprocess(&Config{ExecutablePath: "sh", Args: []string{}}, zap.NewNop())
	require.EqualError(t, subprocess.Shutdown(t.Context()), "no subprocess.cancel().  Has it been started properly?")
}
