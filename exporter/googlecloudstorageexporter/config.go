// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

type Config struct {
	Encoding *component.ID `mapstructure:"encoding"`
	Bucket   bucketConfig  `mapstructure:"bucket"`
}

type bucketConfig struct {
	// ProjectID defines the project where the bucket will be created or
	// where it exists.
	ProjectID string `mapstructure:"project_id"`

	// Name for the bucket storage.
	Name string `mapstructure:"name"`

	// FilePrefix holds the prefix for the created files. FilePrefix can
	// include folders as well. All files will have a suffix that is defined
	// by the exporter.
	// Example:
	// 		filename: folder/file
	//	Files will be placed in 'folder', and will have the prefix 'file'.
	FilePrefix string `mapstructure:"file_prefix"`

	// ReuseIfExists decides if the bucket should be used if it already
	// exists. If it is set to false, an error will be thrown if the
	// bucket already exists. Otherwise, the existent bucket will be
	// used.
	ReuseIfExists bool `mapstructure:"reuse_if_exists"`

	// Region where bucket will be created or where it exists.
	Region string `mapstructure:"region"`
}

var _ xconfmap.Validator = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{
		Bucket: bucketConfig{
			ReuseIfExists: false,
			FilePrefix:    "logs",
		},
	}
}

func (c *bucketConfig) Validate() error {
	var errs []error
	if c.ProjectID == "" {
		errs = append(errs, errors.New("project_id is required"))
	}
	if c.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if c.Region == "" {
		errs = append(errs, errors.New("region is required"))
	}
	return errors.Join(errs...)
}

func (c *Config) Validate() error {
	if c.Encoding == nil {
		return errors.New("encoding is required")
	}
	return nil
}
