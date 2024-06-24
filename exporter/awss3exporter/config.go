// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.uber.org/multierr"
)

// S3UploaderConfig contains aws s3 uploader related config to controls things
// like bucket, prefix, batching, connections, retries, etc.
type S3UploaderConfig struct {
	Region           string                 `mapstructure:"region"`
	S3Bucket         string                 `mapstructure:"s3_bucket"`
	S3Prefix         string                 `mapstructure:"s3_prefix"`
	S3Partition      string                 `mapstructure:"s3_partition"`
	FilePrefix       string                 `mapstructure:"file_prefix"`
	Endpoint         string                 `mapstructure:"endpoint"`
	RoleArn          string                 `mapstructure:"role_arn"`
	S3ForcePathStyle bool                   `mapstructure:"s3_force_path_style"`
	DisableSSL       bool                   `mapstructure:"disable_ssl"`
	Compression      configcompression.Type `mapstructure:"compression"`
}

type MarshalerType string

const (
	OtlpProtobuf MarshalerType = "otlp_proto"
	OtlpJSON     MarshalerType = "otlp_json"
	SumoIC       MarshalerType = "sumo_ic"
	Body         MarshalerType = "body"
)

// Config contains the main configuration options for the s3 exporter
type Config struct {
	S3Uploader    S3UploaderConfig `mapstructure:"s3uploader"`
	MarshalerName MarshalerType    `mapstructure:"marshaler"`

	// Encoding to apply. If present, overrides the marshaler configuration option.
	Encoding              *component.ID `mapstructure:"encoding"`
	EncodingFileExtension string        `mapstructure:"encoding_file_extension"`
}

func (c *Config) Validate() error {
	var errs error
	if c.S3Uploader.Region == "" {
		errs = multierr.Append(errs, errors.New("region is required"))
	}
	if c.S3Uploader.S3Bucket == "" && c.S3Uploader.Endpoint == "" {
		errs = multierr.Append(errs, errors.New("bucket or endpoint is required"))
	}
	compression := c.S3Uploader.Compression
	if compression.IsCompressed() {
		if compression != configcompression.TypeGzip {
			errs = multierr.Append(errs, errors.New("unknown compression type"))
		}

		if c.MarshalerName == SumoIC {
			errs = multierr.Append(errs, errors.New("marshaler does not support compression"))
		}
	}
	return errs
}
