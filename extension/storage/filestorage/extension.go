// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"
)

type localFileStorage struct {
	cfg    *Config
	logger *zap.Logger
}

// Ensure this storage extension implements the appropriate interface
var _ storage.Extension = (*localFileStorage)(nil)

func newLocalFileStorage(logger *zap.Logger, config *Config) (extension.Extension, error) {
	return &localFileStorage{
		cfg:    config,
		logger: logger,
	}, nil
}

// Start does nothing
func (lfs *localFileStorage) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown will close any open databases
func (lfs *localFileStorage) Shutdown(context.Context) error {
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
	client, err := newClient(lfs.logger, absoluteName, lfs.cfg.Timeout, lfs.cfg.Compaction)

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
	// Specify unsafe characters by a negation of allowed characters:
	// - uppercase and lowercase letters A-Z, a-z
	// - digits 0-9
	// - dot `.`
	// - hyphen `-`
	// - underscore `_`
	// - tilde `~`
	unsafeCharactersRegex := regexp.MustCompile(`[^A-Za-z0-9.\-_~]`)

	// Replace all characters other than the above with the tilde `~`
	return unsafeCharactersRegex.ReplaceAllString(name, "~")
}
