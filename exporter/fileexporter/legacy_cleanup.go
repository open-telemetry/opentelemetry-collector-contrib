// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// lumberjackTimeFormat is the timestamp layout used by natefinch/lumberjack
// when naming rotated backup files.  It is identical to the layout used by
// timberjack, so we reuse the same constant value here to keep them in sync.
const lumberjackTimeFormat = "2006-01-02T15-04-05.000"

// migrateLegacyBackups renames lumberjack-format rotated backup files to the
// timberjack naming convention by appending a "-size" reason suffix.
//
// The lumberjack filename pattern is:
//
//	<basename>-<timestamp><ext>
//	e.g. data-2025-01-02T15-04-05.000.jsonl
//
// The timberjack equivalent is:
//
//	<basename>-<timestamp>-size<ext>
//	e.g. data-2025-01-02T15-04-05.000-size.jsonl
//
// After renaming, timberjack's own cleanup logic recognizes the files and
// enforces max_backups and max_days normally — including during runtime
// rotations, not just at startup.  This means the migration only needs to run
// once per exporter start; subsequent cleanup is handled transparently by
// timberjack.
//
// The "-size" suffix is used because lumberjack only supported size-based
// rotation, making it the semantically correct reason for all legacy files.
//
// This function is a no-op when rotation is nil, so it is safe to call
// unconditionally.
func migrateLegacyBackups(path string, rotation *Rotation, logger *zap.Logger) error {
	if rotation == nil {
		return nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	prefix := base[:len(base)-len(ext)] + "-"

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("migrating legacy backups: reading directory %s: %w", dir, err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ts, ok := parseLumberjackTimestamp(name, prefix, ext, rotation.LocalTime)
		if !ok {
			continue
		}

		// Build the timberjack-format target name.
		// Use the same timestamp string that was in the original filename so
		// that timberjack can parse it back without any loss of precision.
		loc := time.UTC
		if rotation.LocalTime {
			loc = time.Local
		}
		tsStr := ts.In(loc).Format(lumberjackTimeFormat)
		newName := prefix + tsStr + "-size" + ext
		newPath := filepath.Join(dir, newName)

		// Skip if the target already exists (e.g. a previous partial run).
		if _, statErr := os.Stat(newPath); statErr == nil {
			// Target exists — remove the source to avoid leaving a duplicate.
			logger.Debug("Legacy backup already migrated, removing duplicate source",
				zap.String("source", name),
				zap.String("target", newName),
			)
			if rmErr := os.Remove(filepath.Join(dir, name)); rmErr != nil && !os.IsNotExist(rmErr) {
				// Non-fatal: warn and continue so one bad file does not abort
				// the rest of the migration.
				logger.Warn("Failed to remove duplicate legacy backup source",
					zap.String("source", name),
					zap.Error(rmErr),
				)
			}
			continue
		}

		if renameErr := os.Rename(filepath.Join(dir, name), newPath); renameErr != nil {
			// Non-fatal: warn and continue so one bad file does not abort
			// the rest of the migration.
			logger.Warn("Failed to rename legacy backup file",
				zap.String("source", name),
				zap.String("target", newName),
				zap.Error(renameErr),
			)
			continue
		}

		logger.Debug("Renamed legacy lumberjack backup to timberjack format",
			zap.String("source", name),
			zap.String("target", newName),
		)
	}

	return nil
}

// parseLumberjackTimestamp returns the rotation time encoded in a lumberjack
// backup filename and true, or the zero time and false if the file does not
// match the lumberjack pattern.
//
// Lumberjack pattern: <prefix><timestamp><ext>
// e.g. "data-2025-01-02T15-04-05.000.jsonl"
//
// The function distinguishes lumberjack files from timberjack files by
// verifying that the part between prefix and ext parses exactly as a
// timestamp.  Timberjack files have an extra "-<reason>" segment after the
// timestamp which prevents an exact parse.
func parseLumberjackTimestamp(name, prefix, ext string, localTime bool) (time.Time, bool) {
	if !strings.HasPrefix(name, prefix) {
		return time.Time{}, false
	}
	if !strings.HasSuffix(name, ext) {
		return time.Time{}, false
	}

	// The middle part should be just the timestamp with no additional segments.
	middle := name[len(prefix) : len(name)-len(ext)]

	loc := time.UTC
	if localTime {
		loc = time.Local
	}

	t, err := time.ParseInLocation(lumberjackTimeFormat, middle, loc)
	if err != nil {
		// Does not parse as a bare timestamp — either a timberjack file
		// (has "-reason") or an unrelated file.
		return time.Time{}, false
	}
	return t, true
}
