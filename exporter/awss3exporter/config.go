// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

// S3UploaderConfig contains aws s3 uploader related config to controls things
// like bucket, prefix, batching, connections, retries, etc.
type S3UploaderConfig struct {
	Region      string `mapstructure:"region"`
	S3Bucket    string `mapstructure:"s3_bucket"`
	S3Prefix    string `mapstructure:"s3_prefix"`
	S3Partition string `mapstructure:"s3_partition"`
	FilePrefix  string `mapstructure:"file_prefix"`
	Endpoint    string `mapstructure:"endpoint"`
}

type MarshalerType string

const (
	OtlpJSON MarshalerType = "otlp_json"
)

// Config contains the main configuration options for the s3 exporter
type Config struct {
	S3Uploader    S3UploaderConfig `mapstructure:"s3uploader"`
	MarshalerName MarshalerType    `mapstructure:"marshaler"`

	FileFormat string `mapstructure:"file_format"`
}
