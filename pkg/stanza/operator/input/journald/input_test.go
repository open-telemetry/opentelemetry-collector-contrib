// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package journald

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type fakeJournaldCmd struct {
	startError error
	exitError  *exec.ExitError
	stdErr     string
}

// fakeJournaldCmdCustom is like fakeJournaldCmd but with a configurable stdout response.
type fakeJournaldCmdCustom struct {
	response string
	stdErr   string
}

func (*fakeJournaldCmdCustom) Start() error { return nil }

func (f *fakeJournaldCmdCustom) StdoutPipe() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte(f.response))), nil
}

func (f *fakeJournaldCmdCustom) StderrPipe() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte(f.stdErr))), nil
}

func (*fakeJournaldCmdCustom) Wait() error { return nil }

func (f *fakeJournaldCmd) Start() error {
	return f.startError
}

func (*fakeJournaldCmd) StdoutPipe() (io.ReadCloser, error) {
	response := `{ "_BOOT_ID": "c4fa36de06824d21835c05ff80c54468", "_CAP_EFFECTIVE": "0", "_TRANSPORT": "journal", "_UID": "1000", "_EXE": "/usr/lib/systemd/systemd", "_AUDIT_LOGINUID": "1000", "MESSAGE": "run-docker-netns-4f76d707d45f.mount: Succeeded.", "_PID": "13894", "_CMDLINE": "/lib/systemd/systemd --user", "_MACHINE_ID": "d777d00e7caf45fbadedceba3975520d", "_SELINUX_CONTEXT": "unconfined\n", "CODE_FUNC": "unit_log_success", "SYSLOG_IDENTIFIER": "systemd", "_HOSTNAME": "myhostname", "MESSAGE_ID": "7ad2d189f7e94e70a38c781354912448", "_SYSTEMD_CGROUP": "/user.slice/user-1000.slice/user@1000.service/init.scope", "_SOURCE_REALTIME_TIMESTAMP": "1587047866229317", "USER_UNIT": "run-docker-netns-4f76d707d45f.mount", "SYSLOG_FACILITY": "3", "_SYSTEMD_SLICE": "user-1000.slice", "_AUDIT_SESSION": "286", "CODE_FILE": "../src/core/unit.c", "_SYSTEMD_USER_UNIT": "init.scope", "_COMM": "systemd", "USER_INVOCATION_ID": "88f7ca6bbf244dc8828fa901f9fe9be1", "CODE_LINE": "5487", "_SYSTEMD_INVOCATION_ID": "83f7fc7799064520b26eb6de1630429c", "PRIORITY": "6", "_GID": "1000", "__REALTIME_TIMESTAMP": "1587047866229555", "_SYSTEMD_UNIT": "user@1000.service", "_SYSTEMD_USER_SLICE": "-.slice", "__CURSOR": "s=b1e713b587ae4001a9ca482c4b12c005;i=1eed30;b=c4fa36de06824d21835c05ff80c54468;m=9f9d630205;t=5a369604ee333;x=16c2d4fd4fdb7c36", "__MONOTONIC_TIMESTAMP": "685540311557", "_SYSTEMD_OWNER_UID": "1000" }
`
	reader := bytes.NewReader([]byte(response))
	return io.NopCloser(reader), nil
}

func (f *fakeJournaldCmd) StderrPipe() (io.ReadCloser, error) {
	reader := bytes.NewReader([]byte(f.stdErr))
	return io.NopCloser(reader), nil
}

func (f *fakeJournaldCmd) Wait() error {
	if f.exitError == nil {
		return nil
	}

	return f.exitError
}

func TestInputJournald(t *testing.T) {
	cfg := NewConfigWithID("my_journald_input")
	cfg.OutputIDs = []string{"output"}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.NewMockOperator("output")
	received := make(chan *entry.Entry)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		received <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = op.SetOutputs([]operator.Operator{mockOutput})
	require.NoError(t, err)

	op.(*Input).newCmd = func(_ context.Context, _ []byte) cmd {
		return &fakeJournaldCmd{}
	}

	require.NoError(t, op.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, op.Stop())
	}()

	expected := map[string]any{
		"_BOOT_ID":                   "c4fa36de06824d21835c05ff80c54468",
		"_CAP_EFFECTIVE":             "0",
		"_TRANSPORT":                 "journal",
		"_UID":                       "1000",
		"_EXE":                       "/usr/lib/systemd/systemd",
		"_AUDIT_LOGINUID":            "1000",
		"MESSAGE":                    "run-docker-netns-4f76d707d45f.mount: Succeeded.",
		"_PID":                       "13894",
		"_CMDLINE":                   "/lib/systemd/systemd --user",
		"_MACHINE_ID":                "d777d00e7caf45fbadedceba3975520d",
		"_SELINUX_CONTEXT":           "unconfined\n",
		"CODE_FUNC":                  "unit_log_success",
		"SYSLOG_IDENTIFIER":          "systemd",
		"_HOSTNAME":                  "myhostname",
		"MESSAGE_ID":                 "7ad2d189f7e94e70a38c781354912448",
		"_SYSTEMD_CGROUP":            "/user.slice/user-1000.slice/user@1000.service/init.scope",
		"_SOURCE_REALTIME_TIMESTAMP": "1587047866229317",
		"USER_UNIT":                  "run-docker-netns-4f76d707d45f.mount",
		"SYSLOG_FACILITY":            "3",
		"_SYSTEMD_SLICE":             "user-1000.slice",
		"_AUDIT_SESSION":             "286",
		"CODE_FILE":                  "../src/core/unit.c",
		"_SYSTEMD_USER_UNIT":         "init.scope",
		"_COMM":                      "systemd",
		"USER_INVOCATION_ID":         "88f7ca6bbf244dc8828fa901f9fe9be1",
		"CODE_LINE":                  "5487",
		"_SYSTEMD_INVOCATION_ID":     "83f7fc7799064520b26eb6de1630429c",
		"PRIORITY":                   "6",
		"_GID":                       "1000",
		"_SYSTEMD_UNIT":              "user@1000.service",
		"_SYSTEMD_USER_SLICE":        "-.slice",
		"__CURSOR":                   "s=b1e713b587ae4001a9ca482c4b12c005;i=1eed30;b=c4fa36de06824d21835c05ff80c54468;m=9f9d630205;t=5a369604ee333;x=16c2d4fd4fdb7c36",
		"__MONOTONIC_TIMESTAMP":      "685540311557",
		"_SYSTEMD_OWNER_UID":         "1000",
	}

	select {
	case e := <-received:
		require.Equal(t, expected, e.Body)
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be read")
	}
}

func TestBuildConfigArgs(t *testing.T) {
	testCases := []struct {
		Name          string
		Config        func(_ *Config)
		Expected      []string
		ExpectedError string
	}{
		{
			Name:     "empty config",
			Config:   func(_ *Config) {},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--priority", "info"},
		},
		{
			Name: "units",
			Config: func(cfg *Config) {
				cfg.Units = []string{
					"dbus.service",
					"user@1000.service",
				}
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--unit", "dbus.service", "--unit", "user@1000.service", "--priority", "info"},
		},
		{
			Name: "matches",
			Config: func(cfg *Config) {
				cfg.Matches = []MatchConfig{
					{
						"_SYSTEMD_UNIT": "dbus.service",
					},
					{
						"_UID":          "1000",
						"_SYSTEMD_UNIT": "user@1000.service",
					},
				}
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--priority", "info", "_SYSTEMD_UNIT=dbus.service", "+", "_SYSTEMD_UNIT=user@1000.service", "_UID=1000"},
		},
		{
			Name: "invalid match",
			Config: func(cfg *Config) {
				cfg.Matches = []MatchConfig{
					{
						"-SYSTEMD_UNIT": "dbus.service",
					},
				}
			},
			ExpectedError: "'-SYSTEMD_UNIT' is not a valid Systemd field name",
		},
		{
			Name: "units and matches",
			Config: func(cfg *Config) {
				cfg.Units = []string{"ssh"}
				cfg.Matches = []MatchConfig{
					{
						"_SYSTEMD_UNIT": "dbus.service",
					},
				}
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--unit", "ssh", "--priority", "info", "_SYSTEMD_UNIT=dbus.service"},
		},
		{
			Name: "identifiers",
			Config: func(cfg *Config) {
				cfg.Identifiers = []string{"wireplumber", "systemd"}
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--identifier", "wireplumber", "--identifier", "systemd", "--priority", "info"},
		},
		{
			Name: "grep",
			Config: func(cfg *Config) {
				cfg.Grep = "test_grep"
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--priority", "info", "--grep", "test_grep"},
		},
		{
			Name: "namespace",
			Config: func(cfg *Config) {
				cfg.Namespace = "foo"
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--priority", "info", "--namespace", "foo"},
		},
		{
			Name: "dmesg",
			Config: func(cfg *Config) {
				cfg.Dmesg = true
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--priority", "info", "--dmesg"},
		},
		{
			Name: "all",
			Config: func(cfg *Config) {
				cfg.All = true
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--priority", "info", "--all"},
		},
		{
			Name: "merge",
			Config: func(cfg *Config) {
				cfg.Merge = true
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--lines=0", "--priority", "info", "--merge"},
		},
		{
			Name: "start_at beginning",
			Config: func(cfg *Config) {
				cfg.StartAt = "beginning"
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--no-tail", "--priority", "info"},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			cfg := NewConfigWithID("my_journald_input")
			tt.Config(cfg)
			args, err := cfg.buildArgs()

			if tt.ExpectedError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.ExpectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.Expected, args)
		})
	}
}

func TestBuildConfigCmd(t *testing.T) {
	testCases := []struct {
		Name       string
		Config     func(_ *Config)
		RequireCmd func(*exec.Cmd)
	}{
		{
			Name:   "empty config",
			Config: func(_ *Config) {},
			RequireCmd: func(cmd *exec.Cmd) {
				require.Nil(t, cmd.SysProcAttr)
				require.NotEmpty(t, cmd.Args)
				assert.Equal(t, "journalctl", cmd.Args[0])
			},
		},
		{
			Name: "custom root_path",
			Config: func(cfg *Config) {
				cfg.RootPath = "/host"
			},
			RequireCmd: func(cmd *exec.Cmd) {
				require.NotNil(t, cmd.SysProcAttr)
				assert.Equal(t, "/host", cmd.SysProcAttr.Chroot)
			},
		},
		{
			Name: "custom journalctl_path",
			Config: func(cfg *Config) {
				cfg.JournalctlPath = "/usr/bin/journalctl"
			},
			RequireCmd: func(cmd *exec.Cmd) {
				require.NotEmpty(t, cmd.Args)
				assert.Equal(t, "/usr/bin/journalctl", cmd.Args[0])
			},
		},
		{
			Name: "custom root_path and journalctl_path",
			Config: func(cfg *Config) {
				cfg.RootPath = "/host"
				cfg.JournalctlPath = "/usr/bin/journalctl"
			},
			RequireCmd: func(cmd *exec.Cmd) {
				require.NotNil(t, cmd.SysProcAttr)
				require.NotEmpty(t, cmd.Args)
				assert.Equal(t, "/host", cmd.SysProcAttr.Chroot)
				// root_path should *not* be prepended to journalctl_path
				assert.Equal(t, "/usr/bin/journalctl", cmd.Args[0])
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			cfg := NewConfigWithID("my_journald_input")
			tt.Config(cfg)
			newCmdFunc, err := cfg.buildNewCmdFunc(zap.NewNop())

			require.NoError(t, err)
			cmd := newCmdFunc(t.Context(), nil).(*exec.Cmd)
			tt.RequireCmd(cmd)
		})
	}
}

func TestBuildConfigCmdCursor(t *testing.T) {
	cfg := NewConfigWithID("my_journald_input")
	newCmdFunc, err := cfg.buildNewCmdFunc(zap.NewNop())
	require.NoError(t, err)

	cmd := newCmdFunc(t.Context(), []byte("cursor-value")).(*exec.Cmd)
	assert.Contains(t, cmd.Args, "--after-cursor")
	assert.Contains(t, cmd.Args, "cursor-value")

	cmd = newCmdFunc(t.Context(), []byte("  ")).(*exec.Cmd)
	assert.NotContains(t, cmd.Args, "--after-cursor")

	cmd = newCmdFunc(t.Context(), []byte{}).(*exec.Cmd)
	assert.NotContains(t, cmd.Args, "--after-cursor")
}

func TestConfigValidation(t *testing.T) {
	testCases := []struct {
		Name          string
		Config        func(_ *Config)
		ExpectedError string
	}{
		{
			Name:   "empty config",
			Config: func(_ *Config) {},
		},
		{
			Name: "invalid journalctl_path",
			Config: func(cfg *Config) {
				cfg.JournalctlPath = " "
			},
			ExpectedError: "'journalctl_path' must be non-whitespace",
		},
		{
			Name: "invalid root_path",
			Config: func(cfg *Config) {
				cfg.RootPath = "not/absolute"
				cfg.JournalctlPath = "/usr/bin/journalctl"
			},
			ExpectedError: "'root_path' must be an absolute path",
		},
		{
			Name: "invalid journalctl_path with valid root_path",
			Config: func(cfg *Config) {
				cfg.RootPath = "/host"
				cfg.JournalctlPath = "journalctl"
			},
			ExpectedError: "'journalctl_path' must be an absolute path when 'root_path' is set",
		},
		{
			Name: "invalid start_at",
			Config: func(cfg *Config) {
				cfg.StartAt = "middle"
			},
			ExpectedError: "invalid value 'middle' for parameter 'start_at'",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			cfg := NewConfigWithID("my_journald_input")
			tt.Config(cfg)
			err := cfg.validate()

			if tt.ExpectedError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.ExpectedError)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestInputJournaldOtelAttributes(t *testing.T) {
	cfg := NewConfigWithID("my_journald_input")
	cfg.OutputIDs = []string{"output"}
	cfg.ConvertToSemanticConventions = true

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.NewMockOperator("output")
	received := make(chan *entry.Entry)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		received <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = op.SetOutputs([]operator.Operator{mockOutput})
	require.NoError(t, err)

	op.(*Input).newCmd = func(_ context.Context, _ []byte) cmd {
		return &fakeJournaldCmd{}
	}

	require.NoError(t, op.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, op.Stop())
	}()

	select {
	case e := <-received:
		// Body must be the MESSAGE string
		assert.Equal(t, "run-docker-netns-4f76d707d45f.mount: Succeeded.", e.Body)

		// Severity must be mapped from PRIORITY "6" → Info
		assert.Equal(t, entry.Info, e.Severity)
		assert.Equal(t, "info", e.SeverityText)

		// OTel log attributes from known journald fields
		require.NotNil(t, e.Attributes)
		assert.Equal(t, "unit_log_success", e.Attributes["code.function.name"])
		assert.Equal(t, "../src/core/unit.c", e.Attributes["code.file.path"])
		assert.Equal(t, int64(5487), e.Attributes["code.line.number"])
		assert.Equal(t, "systemd", e.Attributes["syslog.msg.id"])
		assert.Equal(t, int64(3), e.Attributes["syslog.facility.code"])

		// OTel resource attributes from trusted journald fields
		require.NotNil(t, e.Resource)
		assert.Equal(t, "myhostname", e.Resource["host.name"])
		assert.Equal(t, int64(13894), e.Resource["process.pid"])
		assert.Equal(t, "systemd", e.Resource["process.command"])
		assert.Equal(t, "/usr/lib/systemd/systemd", e.Resource["process.executable.path"])
		assert.Equal(t, "/lib/systemd/systemd --user", e.Resource["process.command_line"])

		// Unmapped fields stay in attributes with original names
		assert.Equal(t, "c4fa36de06824d21835c05ff80c54468", e.Attributes["journald._BOOT_ID"])
		assert.Equal(t, "journal", e.Attributes["journald._TRANSPORT"])
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be read")
	}
}

// newTestOperator creates a started journald Input operator wired to a channel.
// The caller is responsible for calling Stop() after the test.
func newTestOperator(t *testing.T, cfg *Config, mkCmd func(context.Context, []byte) cmd) (chan *entry.Entry, func()) {
	t.Helper()
	cfg.OutputIDs = []string{"output"}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	received := make(chan *entry.Entry, 1)
	mockOutput := testutil.NewMockOperator("output")
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		received <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	require.NoError(t, op.SetOutputs([]operator.Operator{mockOutput}))
	op.(*Input).newCmd = mkCmd

	require.NoError(t, op.Start(testutil.NewUnscopedMockPersister()))
	stop := func() { require.NoError(t, op.Stop()) }
	return received, stop
}

func TestInputJournaldOtelAttributes_EmergencyPriority(t *testing.T) {
	// PRIORITY "0" (emergency) should map to Fatal severity.
	const line = `{"MESSAGE":"system crash","PRIORITY":"0","_HOSTNAME":"box","_PID":"1","_COMM":"init","_EXE":"/sbin/init","_CMDLINE":"/sbin/init","__REALTIME_TIMESTAMP":"1587047866229555","__CURSOR":"s=1"}` + "\n"

	cfg := NewConfigWithID("my_journald_input")
	cfg.ConvertToSemanticConventions = true

	received, stop := newTestOperator(t, cfg, func(_ context.Context, _ []byte) cmd {
		return &fakeJournaldCmdCustom{response: line}
	})
	defer stop()

	select {
	case e := <-received:
		assert.Equal(t, "system crash", e.Body)
		assert.Equal(t, entry.Fatal, e.Severity)
		assert.Equal(t, "emerg", e.SeverityText)
		assert.Equal(t, "box", e.Resource["host.name"])
		assert.Equal(t, int64(1), e.Resource["process.pid"])
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for entry")
	}
}

func TestInputJournaldOtelAttributes_DefaultDisabled(t *testing.T) {
	// Without convert_to_semantic_conventions the full body map must be preserved.
	cfg := NewConfigWithID("my_journald_input")
	// ConvertToSemanticConventions defaults to false — do not set it.

	received, stop := newTestOperator(t, cfg, func(_ context.Context, _ []byte) cmd {
		return &fakeJournaldCmd{}
	})
	defer stop()

	select {
	case e := <-received:
		// Body is a map, not a string
		bodyMap, ok := e.Body.(map[string]any)
		require.True(t, ok, "expected body to be map[string]any when convert_to_semantic_conventions is false")
		assert.Equal(t, "run-docker-netns-4f76d707d45f.mount: Succeeded.", bodyMap["MESSAGE"])
		assert.Equal(t, "6", bodyMap["PRIORITY"])
		// No OTel attributes populated from journald fields
		assert.Empty(t, e.Resource)
		assert.Equal(t, entry.Default, e.Severity)
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for entry")
	}
}

func TestInputJournaldOtelAttributes_ConvertMessageBytesWithOtelAttrs(t *testing.T) {
	// MESSAGE as a byte array should still be converted to a string body when
	// both convert_message_bytes and convert_to_semantic_conventions are true.
	const line = `{"MESSAGE":[116,101,115,116],"PRIORITY":"6","_HOSTNAME":"myhost","_PID":"99","_COMM":"app","_EXE":"/usr/bin/app","_CMDLINE":"/usr/bin/app","__REALTIME_TIMESTAMP":"1587047866229555","__CURSOR":"s=1"}` + "\n"

	cfg := NewConfigWithID("my_journald_input")
	cfg.ConvertMessageBytes = true
	cfg.ConvertToSemanticConventions = true

	received, stop := newTestOperator(t, cfg, func(_ context.Context, _ []byte) cmd {
		return &fakeJournaldCmdCustom{response: line}
	})
	defer stop()

	select {
	case e := <-received:
		// [116,101,115,116] decodes to "test"
		assert.Equal(t, "test", e.Body)
		assert.Equal(t, entry.Info, e.Severity)
		assert.Equal(t, "myhost", e.Resource["host.name"])
		assert.Equal(t, int64(99), e.Resource["process.pid"])
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for entry")
	}
}

func TestInputJournaldOtelAttributes_ErrnoAndThreadID(t *testing.T) {
	// ERRNO and TID should land in attributes as int64.
	const line = `{"MESSAGE":"io error","PRIORITY":"3","ERRNO":"5","TID":"777","_HOSTNAME":"h","_PID":"1","_COMM":"c","_EXE":"/c","_CMDLINE":"c","__REALTIME_TIMESTAMP":"1587047866229555","__CURSOR":"s=1"}` + "\n"

	cfg := NewConfigWithID("my_journald_input")
	cfg.ConvertToSemanticConventions = true

	received, stop := newTestOperator(t, cfg, func(_ context.Context, _ []byte) cmd {
		return &fakeJournaldCmdCustom{response: line}
	})
	defer stop()

	select {
	case e := <-received:
		assert.Equal(t, "5", e.Attributes["journald.ERRNO"])
		assert.Equal(t, int64(777), e.Attributes["thread.id"])
		assert.Equal(t, entry.Error, e.Severity)
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for entry")
	}
}

func TestInputJournaldOtelAttributes_TimestampPreserved(t *testing.T) {
	// Timestamp must be set correctly regardless of convert_to_semantic_conventions.
	const line = `{"MESSAGE":"hello","PRIORITY":"6","_HOSTNAME":"h","_PID":"1","_COMM":"c","_EXE":"/c","_CMDLINE":"c","__REALTIME_TIMESTAMP":"1587047866229555","__CURSOR":"s=1"}` + "\n"

	cfg := NewConfigWithID("my_journald_input")
	cfg.ConvertToSemanticConventions = true

	received, stop := newTestOperator(t, cfg, func(_ context.Context, _ []byte) cmd {
		return &fakeJournaldCmdCustom{response: line}
	})
	defer stop()

	select {
	case e := <-received:
		// __REALTIME_TIMESTAMP 1587047866229555 µs → 1587047866229555000 ns
		assert.Equal(t, int64(1587047866229555000), e.Timestamp.UnixNano())
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for entry")
	}
}

func TestInputJournaldError(t *testing.T) {
	cfg := NewConfigWithID("my_journald_input")
	cfg.OutputIDs = []string{"output"}

	set := componenttest.NewNopTelemetrySettings()
	set.Logger, _ = zap.NewDevelopment()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.NewMockOperator("output")
	received := make(chan *entry.Entry)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		received <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = op.SetOutputs([]operator.Operator{mockOutput})
	require.NoError(t, err)

	op.(*Input).newCmd = func(_ context.Context, _ []byte) cmd {
		return &fakeJournaldCmd{
			exitError:  &exec.ExitError{},
			startError: errors.New("fail to start"),
		}
	}

	err = op.Start(testutil.NewUnscopedMockPersister())
	assert.EqualError(t, err, "journalctl command failed: start journalctl: fail to start")
	require.NoError(t, op.Stop())
}
