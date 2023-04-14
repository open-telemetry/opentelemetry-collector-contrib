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

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

const (
	rotationFieldName = "rotation"
	backupsFieldName  = "max_backups"
)

// Config defines configuration for file exporter.
type Config struct {

	// Path of the file to write to. Path is relative to current directory.
	Path string `mapstructure:"path"`

	// Rotation defines an option about rotation of telemetry files
	Rotation *Rotation `mapstructure:"rotation"`

	// FormatType define the data format of encoded telemetry data
	// Options:
	// - json[default]:  OTLP json bytes.
	// - proto:  OTLP binary protobuf bytes.
	FormatType string `mapstructure:"format"`

	// Compression Codec used to export telemetry data
	// Supported compression algorithms:`zstd`
	Compression string `mapstructure:"compression"`

	// FlushInterval is the duration between flushes.
	// See time.ParseDuration for valid values.
	FlushInterval time.Duration `mapstructure:"flush_interval"`
}

// Rotation an option to rolling log files
type Rotation struct {
	// MaxMegabytes is the maximum size in megabytes of the file before it gets
	// rotated. It defaults to 100 megabytes.
	MaxMegabytes int `mapstructure:"max_megabytes"`

	// MaxDays is the maximum number of days to retain old log files based on the
	// timestamp encoded in their filename.  Note that a day is defined as 24
	// hours and may not exactly correspond to calendar days due to daylight
	// savings, leap seconds, etc. The default is not to remove old log files
	// based on age.
	MaxDays int `mapstructure:"max_days" `

	// MaxBackups is the maximum number of old log files to retain. The default
	// is to 100 files.
	MaxBackups int `mapstructure:"max_backups" `

	// LocalTime determines if the time used for formatting the timestamps in
	// backup files is the computer's local time.  The default is to use UTC
	// time.
	LocalTime bool `mapstructure:"localtime"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Path == "" {
		return errors.New("path must be non-empty")
	}
	if cfg.FormatType != formatTypeJSON && cfg.FormatType != formatTypeProto {
		return errors.New("format type is not supported")
	}
	if cfg.Compression != "" && cfg.Compression != compressionZSTD {
		return errors.New("compression is not supported")
	}
	if cfg.FlushInterval < 0 {
		return errors.New("flush_interval must be larger than zero")
	}
	return nil
}

// Unmarshal a confmap.Conf into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return errors.New("empty config for file exporter")
	}
	// first load the config normally
	err := componentParser.Unmarshal(cfg, confmap.WithErrorUnused())
	if err != nil {
		return err
	}

	// next manually search for protocols in the confmap.Conf,
	// if rotation is not present it means it is disabled.
	if !componentParser.IsSet(rotationFieldName) {
		cfg.Rotation = nil
	}

	// set flush interval to 1 second if not set.
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = time.Second
	}
	return nil
}
