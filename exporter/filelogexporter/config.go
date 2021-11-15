// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filelogexporter

import (
	"errors"

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for file exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// Path to write to. Path is absolute or relative to current working directory.
	Path string `mapstructure:"path"`
	// Base dir attribute key. Base dir will be appended to the path when opening files. This can be used to route logs from different sources to different directories.
	BasedirAttributeKey string `mapstructure:"base_dir_key"`
	// Filename attribute key. If the attribute value is empty, warning will be logged.
	FilenameAttributeKey string `mapstructure:"file_name_key"`
	// Maximum number of opened file. When reached, least recently used file will be closed. A new log entry to a closed file will re-open the file.
	MaxOpenFiles int `mapstructure:"max_open_files"`
}

var _ config.Exporter = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Path == "" {
		return errors.New("path must be non-empty")
	}
	if cfg.FilenameAttributeKey == "" {
		return errors.New("file_name_attr must be non-empty")
	}
	if cfg.MaxOpenFiles < 0 {
		return errors.New("max_open_files should be positive")
	}

	return nil
}
