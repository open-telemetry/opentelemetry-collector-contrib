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
	// where it exists. If it is left empty, it will query the metadata
	// endpoint. It requires the collector to be running in a Google Cloud
	// environment.
	ProjectID string `mapstructure:"project_id"`

	// Name for the bucket storage.
	Name string `mapstructure:"name"`

	// FilePrefix holds the prefix for the created filename.
	// This prefix is applied after the partition path (if any).
	// Example:
	// 		file_prefix: "logs"
	// 		Result: ".../logs_UUID"
	FilePrefix string `mapstructure:"file_prefix"`

	// Partition configures the time-based partitionFormat and file prefix.
	Partition partitionConfig `mapstructure:"partitionFormat"`

	// ReuseIfExists decides if the bucket should be used if it already
	// exists. If it is set to false, an error will be thrown if the
	// bucket already exists. Otherwise, the existent bucket will be
	// used.
	ReuseIfExists bool `mapstructure:"reuse_if_exists"`

	// Region where bucket will be created or where it exists. If it is left
	// empty, it will query the metadata endpoint. It requires the collector
	// to be running in a Google Cloud environment.
	Region string `mapstructure:"region"`
}

type partitionConfig struct {
	// Format is a time format string used to create time-based partitions.
	// If set, the current UTC time formatted with this string will be prepended to
	// the filename. You can use standard strftime format parameters (e.g., %Y, %m, %d, %H, %M, %S).
	// Example: "year=%Y/month=%m/day=%d" would result in "year=2023/month=10/day=25/"
	Format string `mapstructure:"format"`

	// Prefix holds the prefix for the partition path.
	// This prefix is applied before the time-based partition structure.
	// Example:
	// 		prefix: "archive"
	// 		Result: "archive/year=2023/..."
	Prefix string `mapstructure:"prefix"`
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
	if c.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func (*Config) Validate() error {
	return nil
}
