// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

// S3UploaderConfig contains aws s3 uploader related config to controls things
// like bucket, prefix, batching, connections, retries, etc.
type S3UploaderConfig struct {
	Region string `mapstructure:"region"`
	// S3Bucket is the bucket name to be uploaded to.
	S3Bucket string `mapstructure:"s3_bucket"`
	// S3Prefix is the key (directory) prefix to written to inside the bucket
	S3Prefix string `mapstructure:"s3_prefix"`
	// S3Partition is used to provide the rollup on how data is written.
	// Valid values are: [hour,minute]
	S3Partition string `mapstructure:"s3_partition"`
	// FilePrefix is the filename prefix used for the file to avoid any potential collisions.
	FilePrefix string `mapstructure:"file_prefix"`
	// Endpoint is the URL used for communicated with S3.
	Endpoint string `mapstructure:"endpoint"`
	// RoleArn is the role policy to use when interacting with S3
	RoleArn string `mapstructure:"role_arn"`
	// S3ForcePathStyle sets the value for force path style.
	S3ForcePathStyle bool `mapstructure:"s3_force_path_style"`
	// DisableSLL forces communication to happen via HTTP instead of HTTPS.
	DisableSSL bool `mapstructure:"disable_ssl"`
	// Compression sets the algorithm used to process the payload
	// before uploading to S3.
	// Valid values are: `gzip` or no value set.
	Compression configcompression.Type `mapstructure:"compression"`
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
	QueueSettings exporterhelper.QueueConfig `mapstructure:"sending_queue"`

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
