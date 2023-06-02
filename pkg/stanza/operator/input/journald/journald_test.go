// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package journald

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type fakeJournaldCmd struct{}

func (f *fakeJournaldCmd) Start() error {
	return nil
}

func (f *fakeJournaldCmd) StdoutPipe() (io.ReadCloser, error) {
	response := `{ "_BOOT_ID": "c4fa36de06824d21835c05ff80c54468", "_CAP_EFFECTIVE": "0", "_TRANSPORT": "journal", "_UID": "1000", "_EXE": "/usr/lib/systemd/systemd", "_AUDIT_LOGINUID": "1000", "MESSAGE": "run-docker-netns-4f76d707d45f.mount: Succeeded.", "_PID": "13894", "_CMDLINE": "/lib/systemd/systemd --user", "_MACHINE_ID": "d777d00e7caf45fbadedceba3975520d", "_SELINUX_CONTEXT": "unconfined\n", "CODE_FUNC": "unit_log_success", "SYSLOG_IDENTIFIER": "systemd", "_HOSTNAME": "myhostname", "MESSAGE_ID": "7ad2d189f7e94e70a38c781354912448", "_SYSTEMD_CGROUP": "/user.slice/user-1000.slice/user@1000.service/init.scope", "_SOURCE_REALTIME_TIMESTAMP": "1587047866229317", "USER_UNIT": "run-docker-netns-4f76d707d45f.mount", "SYSLOG_FACILITY": "3", "_SYSTEMD_SLICE": "user-1000.slice", "_AUDIT_SESSION": "286", "CODE_FILE": "../src/core/unit.c", "_SYSTEMD_USER_UNIT": "init.scope", "_COMM": "systemd", "USER_INVOCATION_ID": "88f7ca6bbf244dc8828fa901f9fe9be1", "CODE_LINE": "5487", "_SYSTEMD_INVOCATION_ID": "83f7fc7799064520b26eb6de1630429c", "PRIORITY": "6", "_GID": "1000", "__REALTIME_TIMESTAMP": "1587047866229555", "_SYSTEMD_UNIT": "user@1000.service", "_SYSTEMD_USER_SLICE": "-.slice", "__CURSOR": "s=b1e713b587ae4001a9ca482c4b12c005;i=1eed30;b=c4fa36de06824d21835c05ff80c54468;m=9f9d630205;t=5a369604ee333;x=16c2d4fd4fdb7c36", "__MONOTONIC_TIMESTAMP": "685540311557", "_SYSTEMD_OWNER_UID": "1000" }
`
	reader := bytes.NewReader([]byte(response))
	return io.NopCloser(reader), nil
}

func TestInputJournald(t *testing.T) {
	cfg := NewConfigWithID("my_journald_input")
	cfg.OutputIDs = []string{"output"}

	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	mockOutput := testutil.NewMockOperator("output")
	received := make(chan *entry.Entry)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		received <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = op.SetOutputs([]operator.Operator{mockOutput})
	require.NoError(t, err)

	op.(*Input).newCmd = func(ctx context.Context, cursor []byte) cmd {
		return &fakeJournaldCmd{}
	}

	err = op.Start(testutil.NewMockPersister("test"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, op.Stop())
	}()

	expected := map[string]interface{}{
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

func TestBuildConfig(t *testing.T) {
	testCases := []struct {
		Name          string
		Config        func(cfg *Config)
		Expected      []string
		ExpectedError string
	}{
		{
			Name:     "empty config",
			Config:   func(cfg *Config) {},
			Expected: []string{"--utc", "--output=json", "--follow", "--priority", "info"},
		},
		{
			Name: "units",
			Config: func(cfg *Config) {
				cfg.Units = []string{
					"dbus.service",
					"user@1000.service",
				}
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--unit", "dbus.service", "--unit", "user@1000.service", "--priority", "info"},
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
			Expected: []string{"--utc", "--output=json", "--follow", "--priority", "info", "_SYSTEMD_UNIT=dbus.service", "+", "_SYSTEMD_UNIT=user@1000.service", "_UID=1000"},
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
			Expected: []string{"--utc", "--output=json", "--follow", "--unit", "ssh", "--priority", "info", "_SYSTEMD_UNIT=dbus.service"},
		},
		{
			Name: "grep",
			Config: func(cfg *Config) {
				cfg.Grep = "test_grep"
			},
			Expected: []string{"--utc", "--output=json", "--follow", "--priority", "info", "--grep", "test_grep"},
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
