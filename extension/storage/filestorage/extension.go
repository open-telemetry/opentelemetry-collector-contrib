// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

type localFileStorage struct {
	cfg    *Config
	logger *zap.Logger
}

// Ensure this storage extension implements the appropriate interface
var _ storage.Extension = (*localFileStorage)(nil)

func newLocalFileStorage(logger *zap.Logger, config *Config) (extension.Extension, error) {
	if config.CreateDirectory {
		var dirs []string
		if config.Compaction.OnStart || config.Compaction.OnRebound {
			dirs = []string{config.Directory, config.Compaction.Directory}
		} else {
			dirs = []string{config.Directory}
		}
		for _, dir := range dirs {
			if err := ensureDirectoryExists(dir, os.FileMode(config.directoryPermissionsParsed)); err != nil {
				return nil, err
			}
		}
	}
	return &localFileStorage{
		cfg:    config,
		logger: logger,
	}, nil
}

// Start runs cleanup if configured
func (lfs *localFileStorage) Start(context.Context, component.Host) error {
	if lfs.cfg.Compaction.CleanupOnStart {
		return lfs.cleanup(lfs.cfg.Compaction.Directory)
	}
	return nil
}

// Shutdown will close any open databases
func (*localFileStorage) Shutdown(context.Context) error {
	// TODO clean up data files that did not have a client
	// and are older than a threshold (possibly configurable)
	return nil
}

// GetClient returns a storage client for an individual component
func (lfs *localFileStorage) GetClient(_ context.Context, kind component.Kind, ent component.ID, name string) (storage.Client, error) {
	var rawName string
	if name == "" {
		rawName = fmt.Sprintf("%s_%s_%s", kindString(kind), ent.Type(), ent.Name())
	} else {
		rawName = fmt.Sprintf("%s_%s_%s_%s", kindString(kind), ent.Type(), ent.Name(), name)
	}

	rawName = sanitize(rawName)
	absoluteName := filepath.Join(lfs.cfg.Directory, rawName)

	// Try to create client, handling panics if recreate is enabled
	client, err := lfs.createClientWithPanicRecovery(absoluteName)
	if err != nil {
		return nil, err
	}

	// return if compaction is not required
	if lfs.cfg.Compaction.OnStart {
		compactionErr := client.Compact(lfs.cfg.Compaction.Directory, lfs.cfg.Timeout, lfs.cfg.Compaction.MaxTransactionSize)
		if compactionErr != nil {
			lfs.logger.Error("compaction on start failed", zap.Error(compactionErr))
		}
	}

	return client, nil
}

// createClientWithPanicRecovery attempts to create a client, and if recreate is enabled
// and a panic occurs (typically due to database corruption), it will rename the file
// and try again with a fresh database
func (lfs *localFileStorage) createClientWithPanicRecovery(absoluteName string) (client *fileStorageClient, err error) {
	// First attempt: try to create client normally
	if !lfs.cfg.Recreate {
		// If recreate is disabled, just try once
		return newClient(lfs.logger, absoluteName, lfs.cfg.Timeout, lfs.cfg.Compaction, !lfs.cfg.FSync)
	}

	// If recreate is enabled, handle potential panics during database opening
	defer func() {
		if r := recover(); r != nil {
			lfs.logger.Warn("Database corruption detected, recreating database file",
				zap.String("file", absoluteName),
				zap.Any("panic", r))

			// Rename the corrupted file with ISO 8601 timestamp
			timestamp := time.Now().Format("2006-01-02T15:04:05.000")
			backupName := absoluteName + "." + timestamp + ".backup"
			if renameErr := os.Rename(absoluteName, backupName); renameErr != nil {
				err = fmt.Errorf("error renaming corrupted database. Please remove %s manually: %w", absoluteName, renameErr)
				return
			}

			lfs.logger.Info("Corrupted database file renamed",
				zap.String("original", absoluteName),
				zap.String("backup", backupName))

			// Try to create client again with fresh database
			client, err = newClient(lfs.logger, absoluteName, lfs.cfg.Timeout, lfs.cfg.Compaction, !lfs.cfg.FSync)
		}
	}()

	// Try to create the client normally first
	client, err = newClient(lfs.logger, absoluteName, lfs.cfg.Timeout, lfs.cfg.Compaction, !lfs.cfg.FSync)
	return client, err
}

func kindString(k component.Kind) string {
	switch k {
	case component.KindReceiver:
		return "receiver"
	case component.KindProcessor:
		return "processor"
	case component.KindExporter:
		return "exporter"
	case component.KindExtension:
		return "extension"
	case component.KindConnector:
		return "connector"
	default:
		return "other" // not expected
	}
}

// sanitize replaces characters in name that are not safe in a file path
func sanitize(name string) string {
	// Replace all unsafe characters with a tilde followed by the unsafe character's Unicode hex number.
	// https://en.wikipedia.org/wiki/List_of_Unicode_characters
	// For example, the slash is replaced with "~002F", and the tilde itself is replaced with "~007E".
	// We perform replacement on the tilde even though it is a safe character to make sure that the sanitized component name
	// never overlaps with a component name that does not require sanitization.
	var sanitized strings.Builder
	for _, character := range name {
		if isSafe(character) {
			sanitized.WriteString(string(character))
		} else {
			sanitized.WriteString(fmt.Sprintf("~%04X", character))
		}
	}
	return sanitized.String()
}

func isSafe(character rune) bool {
	// Safe characters are the following:
	// - uppercase and lowercase letters A-Z, a-z
	// - digits 0-9
	// - dot `.`
	// - hyphen `-`
	// - underscore `_`
	switch {
	case character >= 'a' && character <= 'z',
		character >= 'A' && character <= 'Z',
		character >= '0' && character <= '9',
		character == '.',
		character == '-',
		character == '_':
		return true
	}
	return false
}

func ensureDirectoryExists(path string, perm os.FileMode) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, perm)
	}
	// we already handled other errors in config.Validate(), so it's okay to return nil
	return nil
}

// cleanup left compaction temporary files from previous killed process
func (lfs *localFileStorage) cleanup(compactionDirectory string) error {
	pattern := filepath.Join(compactionDirectory, fmt.Sprintf("%s*", TempDbPrefix))
	contents, err := filepath.Glob(pattern)
	if err != nil {
		lfs.logger.Info("cleanup error listing temporary files",
			zap.Error(err))
		return err
	}

	var errs []error
	for _, item := range contents {
		err = os.Remove(item)
		if err == nil {
			lfs.logger.Debug("cleanup",
				zap.String("deletedFile", item))
		} else {
			errs = append(errs, err)
		}
	}
	if errs != nil {
		lfs.logger.Info("cleanup errors",
			zap.Error(errors.Join(errs...)))
	}
	return nil
}
