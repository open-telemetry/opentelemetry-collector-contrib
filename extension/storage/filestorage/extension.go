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
// and corruption is detected, it will rename the file and try again with a fresh database.
// Since bbolt panics can occur in internal goroutines that we can't catch with recover(),
// we run the client creation in a separate goroutine to catch those panics.
func (lfs *localFileStorage) createClientWithPanicRecovery(absoluteName string) (client *fileStorageClient, err error) {
	// First attempt: try to create client normally
	if !lfs.cfg.Recreate {
		// If recreate is disabled, just try once
		return newClient(lfs.logger, absoluteName, lfs.cfg.Timeout, lfs.cfg.Compaction, !lfs.cfg.FSync)
	}

	// Try to create the client normally first, with goroutine panic recovery
	client, err = lfs.tryCreateClientWithPanicRecovery(absoluteName)
	
	// If we detect corruption, rename the file and try again
	if err != nil && lfs.isCorruptionError(err) {
		lfs.logger.Warn("Database corruption detected, recreating database file", 
			zap.String("file", absoluteName),
			zap.Error(err))
		
		// Rename the corrupted file
		backupName := absoluteName + ".backup"
		if renameErr := os.Rename(absoluteName, backupName); renameErr != nil {
			// If file doesn't exist, that's fine - we can proceed
			if !os.IsNotExist(renameErr) {
				return nil, fmt.Errorf("error renaming corrupted database. Please remove %s manually: %w", absoluteName, renameErr)
			}
		}
		
		lfs.logger.Info("Corrupted database file renamed, creating fresh database", 
			zap.String("original", absoluteName),
			zap.String("backup", backupName))
		
		// Try to create client again with fresh database
		client, err = lfs.tryCreateClientWithPanicRecovery(absoluteName)
		if err != nil {
			return nil, fmt.Errorf("failed to create fresh database after corruption recovery: %w", err)
		}
	}
	
	return client, err
}

// tryCreateClientWithPanicRecovery attempts to create a client with panic recovery
func (lfs *localFileStorage) tryCreateClientWithPanicRecovery(absoluteName string) (*fileStorageClient, error) {
	type result struct {
		client *fileStorageClient
		err    error
	}
	
	resultChan := make(chan result, 1)
	
	// Run newClient in a goroutine to catch panics from bbolt's internal goroutines
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// If there's a panic in this goroutine, convert it to an error
				lfs.logger.Error("Panic detected during database opening", 
					zap.String("file", absoluteName),
					zap.Any("panic", r))
				resultChan <- result{nil, fmt.Errorf("database corruption panic: %v", r)}
			}
		}()
		
		client, err := newClient(lfs.logger, absoluteName, lfs.cfg.Timeout, lfs.cfg.Compaction, !lfs.cfg.FSync)
		resultChan <- result{client, err}
	}()
	
	// Wait for result
	res := <-resultChan
	return res.client, res.err
}

// isCorruptionError analyzes the error to determine if it indicates database corruption
func (lfs *localFileStorage) isCorruptionError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// Check for known corruption indicators
	corruptionKeywords := []string{
		"corruption",
		"corrupted", 
		"invalid",
		"assertion failed",
		"panic",
		"unexpected",
		"malformed",
		"bad",
		"freelist",
		"page",
		"bucket",
		"meta",
	}
	
	for _, keyword := range corruptionKeywords {
		if strings.Contains(errStr, keyword) {
			lfs.logger.Debug("Corruption keyword detected", 
				zap.String("keyword", keyword),
				zap.Error(err))
			return true
		}
	}
	
	return false
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
