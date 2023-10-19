// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"
)

var replaceUnsafeCharactersFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"extension.filestorage.replaceUnsafeCharacters",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, characters that are not safe in file paths are replaced in component name using the extension. For example, the data for component `filelog/logs/json` will be stored in file `receiver_filelog_logs~007Ejson` and not in `receiver_filelog_logs/json`."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/3148"),
	featuregate.WithRegisterFromVersion("v0.87.0"),
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

	if replaceUnsafeCharactersFeatureGate.IsEnabled() {
		rawName = sanitize(rawName)
	}
	absoluteName := filepath.Join(lfs.cfg.Directory, rawName)
	client, err := newClient(lfs.logger, absoluteName, lfs.cfg.Timeout, lfs.cfg.Compaction, lfs.cfg.FSync)

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
	// Replace all unsafe characters with a tilde followed by the unsafe character's Unicode hex number.
	// https://en.wikipedia.org/wiki/List_of_Unicode_characters
	// For example, the slash is replaced with "~002F", and the tilde itself is replaced with "~007E".
	// We perform replacement on the tilde even though it is a safe character to make sure that the sanitized component name
	// never overlaps with a component name that does not reqire sanitization.
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
