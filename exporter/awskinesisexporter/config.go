// Copyright 2019 OpenTelemetry Authors
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

package awskinesisexporter

import (
	"go.opentelemetry.io/collector/config"
)

// AWSConfig contains AWS specific configuration such as awskinesis stream, region, etc.
type AWSConfig struct {
	StreamName      string `mapstructure:"stream_name"`
	KinesisEndpoint string `mapstructure:"awskinesis_endpoint"`
	Region          string `mapstructure:"region"`
	Role            string `mapstructure:"role"`
}

// KPLConfig contains awskinesis producer library related config to controls things
// like aggregation, batching, connections, retries, etc.
type KPLConfig struct {
	AggregateBatchCount  int `mapstructure:"aggregate_batch_count"`
	AggregateBatchSize   int `mapstructure:"aggregate_batch_size"`
	BatchSize            int `mapstructure:"batch_size"`
	BatchCount           int `mapstructure:"batch_count"`
	BacklogCount         int `mapstructure:"backlog_count"`
	FlushIntervalSeconds int `mapstructure:"flush_interval_seconds"`
	MaxConnections       int `mapstructure:"max_connections"`
	MaxRetries           int `mapstructure:"max_retries"`
	MaxBackoffSeconds    int `mapstructure:"max_backoff_seconds"`
}

// Config contains the main configuration options for the awskinesis exporter
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`

	AWS AWSConfig `mapstructure:"aws"`
	KPL KPLConfig `mapstructure:"kpl"`

	QueueSize            int `mapstructure:"queue_size"`
	NumWorkers           int `mapstructure:"num_workers"`
	MaxBytesPerBatch     int `mapstructure:"max_bytes_per_batch"`
	MaxBytesPerSpan      int `mapstructure:"max_bytes_per_span"`
	FlushIntervalSeconds int `mapstructure:"flush_interval_seconds"`
}
