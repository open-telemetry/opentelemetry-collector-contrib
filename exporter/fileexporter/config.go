// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

const (
	rotationFieldName = "rotation"
	backupsFieldName  = "max_backups"
)

var (
	errInvalidOctal          = errors.New("directory_permissions value must be a valid octal representation")
	errInvalidPermissionBits = errors.New("directory_permissions contain invalid bits for file access")
	errDirPermsRequireCreate = errors.New("directory_permissions requires create_directory to be true")
)

// Config defines configuration for file exporter.
type Config struct {
	// Path of the file to write to. Path is relative to current directory.
	Path string `mapstructure:"path"`

	// Mode defines whether the exporter should append to the file.
	// Options:
	// - false[default]:  truncates the file
	// - true:  appends to the file.
	Append bool `mapstructure:"append"`

	// Rotation defines an option about rotation of telemetry files.
	Rotation *Rotation `mapstructure:"rotation"`

	// FormatType define the data format of encoded telemetry data
	// Options:
	// - json[default]:  OTLP json bytes.
	// - proto:  OTLP binary protobuf bytes.
	FormatType string `mapstructure:"format"`

	// Encoding defines the encoding of the telemetry data.
	// If specified, it overrides `FormatType` and applies an encoding extension.
	Encoding *component.ID `mapstructure:"encoding"`

	// Compression Codec used to export telemetry data
	// Supported compression algorithms:`zstd`
	Compression string `mapstructure:"compression"`

	// FlushInterval is the duration between flushes.
	// See time.ParseDuration for valid values.
	FlushInterval time.Duration `mapstructure:"flush_interval"`

	// GroupBy enables writing to separate files based on a resource attribute.
	GroupBy *GroupBy `mapstructure:"group_by"`

	// CreateDirectory specifies that the parent directory of the output file should be created automatically on start.
	CreateDirectory bool `mapstructure:"create_directory"`
	// DirectoryPermissions specifies permissions used when creating directories (minus process umask).
	// Value must be an octal string like "0755".
	DirectoryPermissions       string `mapstructure:"directory_permissions"`
	directoryPermissionsParsed int64  `mapstructure:"-"`
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

type GroupBy struct {
	// Enables group_by. Default is false.
	Enabled bool `mapstructure:"enabled"`

	// ResourceAttribute specifies the name of the resource attribute that
	// contains the path segment of the file to write to. The final path will be
	// the Path config value, with the * replaced with the value of this resource
	// attribute. Default is "fileexporter.path_segment".
	ResourceAttribute string `mapstructure:"resource_attribute"`

	// MaxOpenFiles specifies the maximum number of open file descriptors for the output files.
	// The default is 100.
	MaxOpenFiles int `mapstructure:"max_open_files"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Path == "" {
		return errors.New("path must be non-empty")
	}
	if cfg.Append && cfg.Compression != "" {
		return errors.New("append and compression enabled at the same time is not supported")
	}
	if cfg.Append && cfg.Rotation != nil {
		return errors.New("append and rotation enabled at the same time is not supported")
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

	if cfg.GroupBy != nil && cfg.GroupBy.Enabled {
		pathParts := strings.Split(cfg.Path, "*")
		if len(pathParts) != 2 {
			return errors.New("path must contain exactly one * when group_by is enabled")
		}

		if pathParts[0] == "" {
			return errors.New("path must not start with * when group_by is enabled")
		}

		if cfg.GroupBy.ResourceAttribute == "" {
			return errors.New("resource_attribute must not be empty when group_by is enabled")
		}
	}

	// If directory auto-creation is enabled, validate and parse permissions.
	if cfg.CreateDirectory {
		permStr := cfg.DirectoryPermissions
		// Default to 0755 if not provided.
		if permStr == "" {
			permStr = "0755"
			cfg.DirectoryPermissions = permStr
		}
		permissions, err := strconv.ParseInt(permStr, 8, 32)
		if err != nil {
			return errInvalidOctal
		}
		if permissions&int64(os.ModePerm) != permissions {
			return errInvalidPermissionBits
		}
		cfg.directoryPermissionsParsed = permissions
	} else if cfg.DirectoryPermissions != "" {
		// If not creating directories, directory_permissions must not be set.
		return errDirPermsRequireCreate
	}

	return nil
}

// Unmarshal a confmap.Conf into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return errors.New("empty config for file exporter")
	}
	// first load the config normally
	err := componentParser.Unmarshal(cfg)
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
