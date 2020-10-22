// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build integration !windows

package subprocess

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shirou/gopsutil/process"
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

// prepareSubprocess will create a Subprocess based on a temporary script.
// It returns a pointer to the pointer to psutil process info and a closure to set its
// value from the running process once started.
func prepareSubprocess(t *testing.T, conf *Config) (*Subprocess, **process.Process, func() bool) {
	logCore, _ := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	scriptFile, err := ioutil.TempFile("", "subproc")
	require.NoError(t, err)

	t.Cleanup(func() { os.Remove(scriptFile.Name()) })

	_, err = scriptFile.Write([]byte(scriptContents))
	require.NoError(t, err)
	require.NoError(t, scriptFile.Chmod(0700))
	scriptFile.Close()

	conf.ExecutablePath = scriptFile.Name()
	subprocess := NewSubprocess(conf, logger)

	selfPid := int32(os.Getpid())
	expectedExecutable := fmt.Sprintf("/bin/sh %v", scriptFile.Name())

	var procInfo *process.Process
	findProcessInfo := func() bool {
		pid := int32(subprocess.Pid())
		if pid != -1 {
			if proc, err := process.NewProcess(int32(subprocess.Pid())); err == nil {
				if ppid, err := proc.Ppid(); err == nil && ppid == selfPid {
					if cmdline, err := proc.Cmdline(); err == nil && strings.HasPrefix(cmdline, expectedExecutable) {
						procInfo = proc
						return true
					}
				}
			}
		}
		return false
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

func TestHappyPathIntegration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subprocess, procInfo, findProcessInfo := prepareSubprocess(t, &Config{})
	subprocess.Start(ctx)
	defer subprocess.Shutdown(ctx)

	require.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	cmdline, err := (*procInfo).Cmdline()
	require.NoError(t, err)
	require.Equal(t, "/bin/sh "+subprocess.config.ExecutablePath, cmdline)
}

func TestWithArgsIntegration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subprocess, procInfo, findProcessInfo := prepareSubprocess(t, &Config{Args: []string{"myArgs"}})
	subprocess.Start(ctx)
	defer subprocess.Shutdown(ctx)

	require.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	cmdline, err := (*procInfo).Cmdline()
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("/bin/sh %v myArgs", subprocess.config.ExecutablePath), cmdline)
}

func TestWithEnvVarsIntegration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &Config{
		EnvironmentVariables: map[string]string{
			"MyEnv1": "MyVal1",
			"MyEnv2": "MyVal2",
		},
	}

	subprocess, procInfo, findProcessInfo := prepareSubprocess(t, config)
	subprocess.Start(ctx)
	defer subprocess.Shutdown(ctx)
	require.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	stdout := <-subprocess.Stdout
	require.NotEmpty(t, stdout)
	require.Contains(t, stdout, "MyEnv1=MyVal1")
	require.Contains(t, stdout, "MyEnv2=MyVal2")
}

func TestWithAutoRestartIntegration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	restartDelay := 100 * time.Millisecond
	subprocess, procInfo, findProcessInfo := prepareSubprocess(t, &Config{RestartOnError: true, RestartDelay: &restartDelay})
	subprocess.Start(ctx)
	defer subprocess.Shutdown(ctx)

	require.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	cmdline, err := (*procInfo).Cmdline()
	require.NoError(t, err)
	require.Equal(t, "/bin/sh "+subprocess.config.ExecutablePath, cmdline)

	oldProcPid := (*procInfo).Pid
	err = (*procInfo).Kill()
	require.NoError(t, err)

	// Should be restarted
	require.Eventually(t, findProcessInfo, restartDelay+5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	require.NotEqual(t, (*procInfo).Pid, oldProcPid)
}

func TestSendingStdinIntegration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subprocess, procInfo, findProcessInfo := prepareSubprocess(t, &Config{StdInContents: "mystdincontents"})
	subprocess.Start(ctx)
	defer subprocess.Shutdown(ctx)

	require.Eventually(t, findProcessInfo, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, *procInfo)

	requireDesiredStdout(t, subprocess, "Stdin: mystdincontents")
}

func TestSendingStdinFailsIntegration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logCore, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	subprocess := NewSubprocess(&Config{ExecutablePath: "echo", Args: []string{"finished"}}, logger)

	intentionalError := fmt.Errorf("intentional failure")
	subprocess.sendToStdIn = func(contents string, writer io.Writer) error {
		return intentionalError
	}

	subprocess.Start(ctx)
	defer subprocess.Shutdown(ctx)

	matched := func() bool {
		died := len(logObserver.FilterMessage("subprocess died").All()) == 1
		errored := len(logObserver.FilterField(zap.Error(intentionalError)).All()) == 1
		return died && errored
	}

	require.Eventually(t, matched, 10*time.Second, 10*time.Millisecond)
}

func TestSubprocessBadExecIntegration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logCore, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	subprocess := NewSubprocess(&Config{ExecutablePath: "/does/not/exist"}, logger)
	subprocess.Start(ctx)
	defer subprocess.Shutdown(ctx)

	matched := func() bool {
		return len(logObserver.FilterMessage("subprocess died").All()) == 1
	}

	require.Eventually(t, matched, 10*time.Second, 10*time.Millisecond)
}

func TestSubprocessSuccessfullyReturnsIntegration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// There is a race condition between writing from the stdout scanner and the closing of the stdout channel on
	// process exit. Here we sleep before returning, but this will need to be addressed if short lived processes
	// become an intended use case without forcing users to read stdout before closing.
	subprocess := NewSubprocess(&Config{ExecutablePath: "sh", Args: []string{"-c", "echo finished; sleep .1"}}, zap.NewNop())
	subprocess.Start(ctx)
	defer subprocess.Shutdown(ctx)

	matched := func() bool {
		_, ok := <-subprocess.shutdownSignal
		return !ok
	}

	require.Eventually(t, matched, 10*time.Second, 10*time.Millisecond)
	requireDesiredStdout(t, subprocess, "finished")
}
