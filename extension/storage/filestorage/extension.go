// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"
)

type localFileStorage struct {
	directory           string
	timeout             time.Duration
	logger              *zap.Logger
	compactionDirectory string
	compactOnStart      bool
	maxCompactionSize   int64
}

// Ensure this storage extension implements the appropriate interface
var _ storage.Extension = (*localFileStorage)(nil)

func newLocalFileStorage(logger *zap.Logger, config *Config) (component.Extension, error) {
	return &localFileStorage{
		directory:           filepath.Clean(config.Directory),
		compactionDirectory: filepath.Clean(config.Compaction.Directory),
		compactOnStart:      config.Compaction.OnStart,
		timeout:             config.Timeout,
		logger:              logger,
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
func (lfs *localFileStorage) GetClient(ctx context.Context, kind component.Kind, ent config.ComponentID, name string) (storage.Client, error) {
	var rawName string
	if name == "" {
		rawName = fmt.Sprintf("%s_%s_%s", kindString(kind), ent.Type(), ent.Name())
	} else {
		rawName = fmt.Sprintf("%s_%s_%s_%s", kindString(kind), ent.Type(), ent.Name(), name)
	}
	// TODO sanitize rawName
	absoluteName := filepath.Join(lfs.directory, rawName)
	client, err := newClient(absoluteName, lfs.timeout)

	// return if compaction is not required
	if err != nil || !lfs.compactOnStart {
		return client, err
	}

	// perform compaction and returns client
	return client.Compact(ctx, lfs.compactionDirectory, lfs.timeout, lfs.maxCompactionSize)
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
	default:
		return "other" // not expected
	}
}
