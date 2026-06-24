// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

type localFileStorage struct {
	cfg    *Config
	logger *zap.Logger

	// precheckFn runs the corruption pre-check for a bbolt database file when
	// Recreate is enabled. It is a struct field so tests can substitute a fake
	// implementation that does not spawn a subprocess.
	precheckFn func(ctx context.Context, path string, timeout time.Duration) error

	// newClientFn opens (or creates) the underlying bbolt client. It is a
	// struct field so tests can substitute a fake that simulates a panic on
	// the first call, allowing the defer/recover path in openClient to be
	// exercised without constructing a real corrupt bbolt file.
	newClientFn func(logger *zap.Logger, filePath string, timeout time.Duration, compactionCfg *CompactionConfig, noSync bool) (*fileStorageClient, error)
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
		cfg:         config,
		logger:      logger,
		precheckFn:  runPrecheck,
		newClientFn: newClient,
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
func (lfs *localFileStorage) GetClient(ctx context.Context, kind component.Kind, ent component.ID, name string) (storage.Client, error) {
	var rawName string
	if name == "" {
		rawName = fmt.Sprintf("%s_%s_%s", kindString(kind), ent.Type(), ent.Name())
	} else {
		rawName = fmt.Sprintf("%s_%s_%s_%s", kindString(kind), ent.Type(), ent.Name(), name)
	}

	rawName = sanitize(rawName)
	absoluteName := filepath.Join(lfs.cfg.Directory, rawName)

	client, err := lfs.openClient(ctx, absoluteName)

	// If the error is due to filename being too long, truncate and try again
	if errors.Is(err, syscall.ENAMETOOLONG) {
		hashedName := filepath.Join(lfs.cfg.Directory, hash(rawName))
		lfs.logger.Warn("filename too long, using hashed filename instead",
			zap.String("originalFile", absoluteName), zap.String("component", rawName), zap.String("hashedFileName", hashedName))
		client, err = lfs.openClient(ctx, hashedName)
	}

	// return error if still not successful
	if err != nil {
		return nil, err
	}

	if lfs.cfg.Compaction.OnStart {
		compactionErr := client.Compact(lfs.cfg.Compaction.Directory, lfs.cfg.Timeout, lfs.cfg.Compaction.MaxTransactionSize)
		if compactionErr != nil {
			lfs.logger.Error("compaction on start failed", zap.Error(compactionErr))
		}
	}

	return client, nil
}

// openClient opens (or creates) the bbolt database for a single client.
//
// When Recreate is enabled and the database file already exists, corruption
// recovery is performed in two layers:
//
//  1. Subprocess pre-check: catches panics raised inside goroutines spawned by
//     bbolt.Open (e.g. the freepages "multiple references" panic), which the
//     in-process defer/recover below cannot catch because Go's recover() only
//     catches panics in its own goroutine.
//  2. In-process defer/recover around bbolt.Open: catches panics raised in the
//     main goroutine (e.g. freepages' "failed to open read only tx" or "failed
//     to rollback tx" panics, or panics from tx.recursivelyCheckBucket itself
//     prior to the spawned goroutine reading from the error channel).
//
// On either signal of corruption, the file is renamed aside and a fresh
// database is opened in its place.
func (lfs *localFileStorage) openClient(ctx context.Context, absoluteName string) (client *fileStorageClient, err error) {
	if lfs.cfg.Recreate {
		if _, statErr := os.Stat(absoluteName); statErr == nil {
			precheckErr := lfs.precheckFn(ctx, absoluteName, lfs.cfg.Timeout)
			switch {
			case precheckErr == nil:
				// Database opened cleanly in the subprocess; safe to open here.
			case errors.Is(precheckErr, errDBCorruption):
				if renameErr := lfs.renameCorruptDB(absoluteName); renameErr != nil {
					return nil, renameErr
				}
			default:
				lfs.logger.Warn("Database pre-check returned non-corruption error; proceeding with open",
					zap.String("file", absoluteName),
					zap.Error(precheckErr))
			}
		}

		defer func() {
			if r := recover(); r != nil {
				lfs.logger.Warn("bbolt.Open panicked in main goroutine; recreating database",
					zap.String("file", absoluteName),
					zap.Any("panic", r))
				if renameErr := lfs.renameCorruptDB(absoluteName); renameErr != nil {
					err = renameErr
					return
				}
				client, err = lfs.newClientFn(lfs.logger, absoluteName, lfs.cfg.Timeout, lfs.cfg.Compaction, !lfs.cfg.FSync)
			}
		}()
	}

	return lfs.newClientFn(lfs.logger, absoluteName, lfs.cfg.Timeout, lfs.cfg.Compaction, !lfs.cfg.FSync)
}

// renameCorruptDB moves a corrupt bbolt database file aside with a timestamped
// suffix so that a fresh database can be created in its place.
func (lfs *localFileStorage) renameCorruptDB(path string) error {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000")
	backup := path + "." + timestamp + ".backup"
	if err := os.Rename(path, backup); err != nil {
		return fmt.Errorf("rename corrupt database %s -> %s (please remove manually): %w", path, backup, err)
	}
	lfs.logger.Warn("Renamed corrupt bbolt database; a fresh database will be created",
		zap.String("original", path),
		zap.String("backup", backup))
	return nil
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
			fmt.Fprint(&sanitized, string(character))
		} else {
			fmt.Fprintf(&sanitized, "~%04X", character)
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

// hash ensures the filename is within filesystem limits.
// On most systems, the maximum file name length is 255 bytes.
// We use a SHA-256 hash to generate a fixed-length filename (64 characters).
func hash(name string) string {
	hashID := sha256.Sum256([]byte(name))
	return hex.EncodeToString(hashID[:]) // filename safe
}
