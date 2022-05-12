// Copyright 2022 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awss3exporter

import (
	"go.opentelemetry.io/collector/config"
	"go.uber.org/zap"
)

// S3UploaderConfig contains aws s3 uploader related config to controls things
// like bucket, prefix, batching, connections, retries, etc.
type S3UploaderConfig struct {
	Region              string `mapstructure:"region"`
	S3Bucket            string `mapstructure:"s3_bucket"`
	S3Prefix            string `mapstructure:"s3_prefix"`
	S3Partition         string `mapstructure:"s3_partition"`
	FilePrefix          string `mapstructure:"file_prefix"`
}

// Config contains the main configuration options for the awskinesis exporter
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`

	FileFormat string           `mapstructure:"file_format"`
	S3Uploader S3UploaderConfig `mapstructure:"s3uploader"`

	// MetricDescriptors is the list of override metric descriptors
	MetricDescriptors []MetricDescriptor `mapstructure:"metric_descriptors"`

	// ResourceToTelemetrySettings is the option for converting resource attrihutes to telemetry attributes.
	// "Enabled" - A boolean field to enable/disable this option. Default is `false`.
	// If enabled, all the resource attributes will be converted to metric labels by default.
	// exporterhelper.ResourceToTelemetrySettings `mapstructure:"resource_to_telemetry_conversion"`

	logger *zap.Logger
}
