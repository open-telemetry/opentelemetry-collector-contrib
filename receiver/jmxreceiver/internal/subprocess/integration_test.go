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
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const scriptContents = `#!/bin/sh
echo "Env:" $(env)
echo "Stdin:" $(cat -)

# this will be canceled and shouldn't influence overall test time
sleep 60
`

type SubprocessIntegrationSuite struct {
	suite.Suite
	scriptPath string
}

func TestSubprocessIntegration(t *testing.T) {
	suite.Run(t, new(SubprocessIntegrationSuite))
}

func (suite *SubprocessIntegrationSuite) SetupSuite() {
	t := suite.T()
	scriptFile, err := os.CreateTemp(t.TempDir(), "subproc")
	require.NoError(t, err)

	_, err = scriptFile.Write([]byte(scriptContents))
	require.NoError(t, err)
	require.NoError(t, scriptFile.Chmod(0o700))
	scriptFile.Close()

	suite.scriptPath = scriptFile.Name()
}

func (suite *SubprocessIntegrationSuite) TearDownSuite() {
	suite.Require().NoError(os.Remove(suite.scriptPath))
}

// prepareSubprocess will create a Subprocess based on a temporary script.
// It returns a pointer to the pointer to psutil process info and a closure to set its
// value from the running process once started.
func (suite *SubprocessIntegrationSuite) prepareSubprocess(conf *Config) (*Subprocess, **process.Process, func() bool) {
	t := suite.T()
	logCore, _ := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	conf.ExecutablePath = suite.scriptPath
	subprocess := NewSubprocess(conf, logger)

	selfPid := int32(os.Getpid())
	expectedExecutable := fmt.Sprintf("/bin/sh %v", suite.scriptPath)

	var procInfo *process.Process
	findProcessInfo := func() bool {
		pid := int32(subprocess.Pid())
		if pid == -1 {
			return false
		}
		proc, err := process.NewProcess(pid)
		require.NoError(t, err)
		ppid, err := proc.Ppid()
		require.NoError(t, err)
		require.Equal(t, selfPid, ppid)

		cmdline, err := proc.Cmdline()
		if cmdline == "" {
			return false
		}
		require.NoError(t, err)
		require.Truef(t, strings.HasPrefix(cmdline, expectedExecutable), "%v doesn't have prefix %v", cmdline, expectedExecutable)
		procInfo = proc
		return true
	}

	return subprocess, &procInfo, findProcessInfo
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

func (suite *SubprocessIntegrationSuite) TestHappyPath() {
	t := suite.T()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subprocess, procInfo, findProcessInfo := suite.prepareSubprocess(&Config{})
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	assert.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	cmdline, err := (*procInfo).Cmdline()
	assert.NoError(t, err)
	assert.Equal(t, "/bin/sh "+subprocess.config.ExecutablePath, cmdline)
}

func (suite *SubprocessIntegrationSuite) TestWithArgs() {
	t := suite.T()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subprocess, procInfo, findProcessInfo := suite.prepareSubprocess(&Config{Args: []string{"myArgs"}})
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	require.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	cmdline, err := (*procInfo).Cmdline()
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("/bin/sh %v myArgs", subprocess.config.ExecutablePath), cmdline)
}

func (suite *SubprocessIntegrationSuite) TestWithEnvVars() {
	t := suite.T()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &Config{
		EnvironmentVariables: map[string]string{
			"MyEnv1": "MyVal1",
			"MyEnv2": "MyVal2",
		},
	}

	subprocess, procInfo, findProcessInfo := suite.prepareSubprocess(config)
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()
	require.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	stdout := <-subprocess.Stdout
	require.NotEmpty(t, stdout)
	require.Contains(t, stdout, "MyEnv1=MyVal1")
	require.Contains(t, stdout, "MyEnv2=MyVal2")
}

func (suite *SubprocessIntegrationSuite) TestWithAutoRestart() {
	t := suite.T()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	restartDelay := 100 * time.Millisecond
	subprocess, procInfo, findProcessInfo := suite.prepareSubprocess(&Config{RestartOnError: true, RestartDelay: &restartDelay})
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	require.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	cmdline, err := (*procInfo).Cmdline()
	require.NoError(t, err)
	require.Equal(t, "/bin/sh "+subprocess.config.ExecutablePath, cmdline)

	oldProcPid := (*procInfo).Pid
	err = (*procInfo).Kill()
	require.NoError(t, err)

	// Should be restarted
	require.Eventually(t, func() bool {
		return findProcessInfo() && *procInfo != nil && (*procInfo).Pid != oldProcPid
	}, restartDelay+5*time.Second, 10*time.Millisecond)
}

func (suite *SubprocessIntegrationSuite) TestSendingStdin() {
	t := suite.T()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subprocess, procInfo, findProcessInfo := suite.prepareSubprocess(&Config{StdInContents: "mystdincontents"})
	assert.NoError(t, subprocess.Start(ctx))
	defer func() {
		assert.NoError(t, subprocess.Shutdown(ctx))
	}()

	require.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	requireDesiredStdout(t, subprocess, "Stdin: mystdincontents")
}

func (suite *SubprocessIntegrationSuite) TestSendingStdinFails() {
	t := suite.T()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logCore, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	subprocess := NewSubprocess(&Config{ExecutablePath: "echo", Args: []string{"finished"}}, logger)

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

	require.Eventually(t, matched, 10*time.Second, 10*time.Millisecond)
}

func (suite *SubprocessIntegrationSuite) TestSubprocessBadExec() {
	t := suite.T()
	ctx, cancel := context.WithCancel(context.Background())
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

	require.Eventually(t, matched, 10*time.Second, 10*time.Millisecond)
}

func (suite *SubprocessIntegrationSuite) TestSubprocessSuccessfullyReturns() {
	t := suite.T()
	ctx, cancel := context.WithCancel(context.Background())
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

	require.Eventually(t, matched, 10*time.Second, 10*time.Millisecond)
	requireDesiredStdout(t, subprocess, "finished")
}

func TestShutdownBeforeStartIntegration(t *testing.T) {
	t.Parallel()
	subprocess := NewSubprocess(&Config{ExecutablePath: "sh", Args: []string{}}, zap.NewNop())
	require.EqualError(t, subprocess.Shutdown(context.Background()), "no subprocess.cancel().  Has it been started properly?")
}
