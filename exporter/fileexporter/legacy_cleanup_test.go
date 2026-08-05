// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DeRuina/timberjack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// createLumberjackFile creates an empty file using the lumberjack backup naming
// pattern: data-<timestamp>.jsonl.
func createLumberjackFile(t *testing.T, dir string, ts time.Time) string {
	t.Helper()
	name := "data-" + ts.UTC().Format(lumberjackTimeFormat) + ".jsonl"
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return path
}

// createTimberjackFile creates an empty file using the timberjack backup naming
// pattern: data-<timestamp>-<reason>.jsonl.
func createTimberjackFile(t *testing.T, dir, reason string, ts time.Time) string {
	t.Helper()
	name := "data-" + ts.UTC().Format(lumberjackTimeFormat) + "-" + reason + ".jsonl"
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return path
}

func TestParseLumberjackTimestamp(t *testing.T) {
	ts := time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)
	prefix := "data-"
	ext := ".jsonl"

	tests := []struct {
		name     string
		filename string
		wantOK   bool
		wantTime time.Time
	}{
		{
			name:     "valid lumberjack file",
			filename: "data-2025-01-02T15-04-05.000.jsonl",
			wantOK:   true,
			wantTime: ts,
		},
		{
			name:     "timberjack file with reason suffix",
			filename: "data-2025-01-02T15-04-05.000-size.jsonl",
			wantOK:   false,
		},
		{
			name:     "unrelated file",
			filename: "data.jsonl",
			wantOK:   false,
		},
		{
			name:     "wrong prefix",
			filename: "other-2025-01-02T15-04-05.000.jsonl",
			wantOK:   false,
		},
		{
			name:     "wrong extension",
			filename: "data-2025-01-02T15-04-05.000.log",
			wantOK:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseLumberjackTimestamp(tc.filename, prefix, ext, false)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantTime, got.UTC())
			}
		})
	}
}

func TestMigrateLegacyBackups_NilRotation(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "data.jsonl")

	ts := time.Now().Add(-1 * time.Hour)
	legacyFile := createLumberjackFile(t, dir, ts)

	require.NoError(t, migrateLegacyBackups(logPath, nil, zap.NewNop()))

	_, err := os.Stat(legacyFile)
	assert.NoError(t, err, "legacy file must not be renamed when rotation is nil")
}

func TestMigrateLegacyBackups_RenamesWithSizeSuffix(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "data.jsonl")

	ts := time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)
	oldPath := createLumberjackFile(t, dir, ts)
	expectedNewPath := filepath.Join(dir, "data-2025-01-02T15-04-05.000-size.jsonl")

	rotation := &Rotation{}
	require.NoError(t, migrateLegacyBackups(logPath, rotation, zap.NewNop()))

	_, err := os.Stat(oldPath)
	assert.True(t, os.IsNotExist(err), "original lumberjack file must no longer exist")

	_, err = os.Stat(expectedNewPath)
	assert.NoError(t, err, "renamed timberjack-format file must exist")
}

func TestMigrateLegacyBackups_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "data.jsonl")

	now := time.Now()
	ts1 := now.Add(-1 * time.Hour)
	ts2 := now.Add(-2 * time.Hour)

	createLumberjackFile(t, dir, ts1)
	createLumberjackFile(t, dir, ts2)

	rotation := &Rotation{}
	require.NoError(t, migrateLegacyBackups(logPath, rotation, zap.NewNop()))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, e := range entries {
		name := e.Name()
		if name == "data.jsonl" {
			continue
		}
		assert.Contains(t, name, "-size.jsonl",
			"all rotated files must have the timberjack -size suffix after migration: %s", name)
	}
}

func TestMigrateLegacyBackups_TimberjackFilesUntouched(t *testing.T) {
	// All filename patterns that timberjack produces must never be renamed by
	// the migration.  This covers:
	//   - every rotation reason (size, time, custom)
	//   - every compression suffix (.gz, .zst, none)
	//   - the append-after-ext naming variant
	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	tsStr := ts.UTC().Format(lumberjackTimeFormat)

	tests := []struct {
		name     string
		filename string
	}{
		// Standard timberjack pattern: <prefix><timestamp>-<reason><ext>
		{name: "reason=size", filename: "data-" + tsStr + "-size.jsonl"},
		{name: "reason=time", filename: "data-" + tsStr + "-time.jsonl"},
		{name: "reason=custom", filename: "data-" + tsStr + "-myrotation.jsonl"},
		// Compressed variants
		{name: "reason=size, gz", filename: "data-" + tsStr + "-size.jsonl.gz"},
		{name: "reason=size, zst", filename: "data-" + tsStr + "-size.jsonl.zst"},
		{name: "reason=time, gz", filename: "data-" + tsStr + "-time.jsonl.gz"},
		{name: "reason=time, zst", filename: "data-" + tsStr + "-time.jsonl.zst"},
		// Append-after-ext variant: <basename><ext>-<timestamp>-<reason>
		{name: "append-after-ext, reason=size", filename: "data.jsonl-" + tsStr + "-size"},
		{name: "append-after-ext, reason=time", filename: "data.jsonl-" + tsStr + "-time"},
		{name: "append-after-ext, gz", filename: "data.jsonl-" + tsStr + "-size.gz"},
		{name: "append-after-ext, zst", filename: "data.jsonl-" + tsStr + "-size.zst"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "data.jsonl")

			filePath := filepath.Join(dir, tc.filename)
			f, err := os.Create(filePath)
			require.NoError(t, err)
			require.NoError(t, f.Close())

			rotation := &Rotation{}
			require.NoError(t, migrateLegacyBackups(logPath, rotation, zap.NewNop()))

			_, err = os.Stat(filePath)
			assert.NoError(t, err, "timberjack file must not be touched by migration: %s", tc.filename)
		})
	}
}

func TestMigrateLegacyBackups_IdempotentOnRestart(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "data.jsonl")

	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	createLumberjackFile(t, dir, ts)
	expectedPath := filepath.Join(dir, "data-2025-06-01T12-00-00.000-size.jsonl")

	rotation := &Rotation{}

	// First run: renames the file.
	require.NoError(t, migrateLegacyBackups(logPath, rotation, zap.NewNop()))
	_, err := os.Stat(expectedPath)
	require.NoError(t, err)

	// Second run: must not error even though the lumberjack source is gone.
	require.NoError(t, migrateLegacyBackups(logPath, rotation, zap.NewNop()))
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "renamed file must still exist after second migration run")
}

func TestMigrateLegacyBackups_TargetAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "data.jsonl")

	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	// Simulate a partial previous run: both old and new names exist.
	sourcePath := createLumberjackFile(t, dir, ts)
	targetPath := createTimberjackFile(t, dir, "size", ts)

	rotation := &Rotation{}
	require.NoError(t, migrateLegacyBackups(logPath, rotation, zap.NewNop()))

	_, err := os.Stat(sourcePath)
	assert.True(t, os.IsNotExist(err), "duplicate source must be removed when target already exists")

	_, err = os.Stat(targetPath)
	assert.NoError(t, err, "target file must be preserved")
}

func TestMigrateLegacyBackups_NoFiles(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "data.jsonl")

	rotation := &Rotation{}
	assert.NoError(t, migrateLegacyBackups(logPath, rotation, zap.NewNop()))
}

// TestMigrateLegacyBackups_TimberjackRecognizesRenamedFiles verifies the
// critical integration property: after migration, timberjack actually
// recognizes the renamed files and subjects them to max_days cleanup.
//
// This ensures the rename produces filenames that timberjack can parse, not
// just that the rename itself succeeds.
func TestMigrateLegacyBackups_TimberjackRecognizesRenamedFiles(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "data.jsonl")

	// Create a lumberjack-format backup older than max_days so timberjack
	// should delete it once it recognizes the file.
	oldTS := time.Now().Add(-48 * time.Hour)
	legacyFile := createLumberjackFile(t, dir, oldTS)

	// Run the migration — renames data-<ts>.jsonl to data-<ts>-size.jsonl.
	rotation := &Rotation{MaxDays: 1}
	require.NoError(t, migrateLegacyBackups(logPath, rotation, zap.NewNop()))

	// The original lumberjack file must be gone.
	_, err := os.Stat(legacyFile)
	require.True(t, os.IsNotExist(err), "lumberjack source file must have been renamed")

	// Start a timberjack logger with the same path and retention config.
	// Calling Rotate() triggers timberjack's cleanup mill (millRunOnce),
	// which calls oldLogFiles() and removes files that violate max_days.
	logger := &timberjack.Logger{
		Filename:   logPath,
		MaxAge:     rotation.MaxDays,
		MaxBackups: rotation.MaxBackups,
		LocalTime:  rotation.LocalTime,
	}
	require.NoError(t, logger.Rotate())
	require.NoError(t, logger.Close())

	// Timberjack's cleanup mill runs asynchronously after Rotate(). Poll until
	// the renamed file disappears or the deadline is reached.
	tsStr := oldTS.UTC().Format(lumberjackTimeFormat)
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		entries, err := os.ReadDir(dir)
		require.NoError(ct, err)
		for _, e := range entries {
			assert.NotContains(ct, e.Name(), tsStr,
				"timberjack must have cleaned up the renamed legacy file: %s", e.Name())
		}
	}, 2*time.Second, 50*time.Millisecond)
}
