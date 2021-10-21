// Copyright 2020, OpenTelemetry Authors
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

package awscloudwatchlogsexporter

import (
	"errors"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config represent a configuration for the CloudWatch logs exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`

	// LogGroupName is the name of CloudWatch log group which defines group of log streams
	// that share the same retention, monitoring, and access control settings.
	LogGroupName string `mapstructure:"log_group_name"`

	// LogStreamName is the name of CloudWatch log stream which is a sequence of log events
	// that share the same source.
	LogStreamName string `mapstructure:"log_stream_name"`

	// Region is the AWS region where the logs are sent to.
	// Optional.
	Region string `mapstructure:"region"`

	// Endpoint is the CloudWatch Logs service endpoint which the requests
	// are forwarded to. https://docs.aws.amazon.com/general/latest/gr/cwl_region.html
	// e.g. logs.us-east-1.amazonaws.com
	// Optional.
	Endpoint string `mapstructure:"endpoint"`

	// QueueSettings is a subset of exporterhelper.QueueSettings,
	// because only QueueSize is user-settable due to how AWS CloudWatch API works
	QueueSettings QueueSettings `mapstructure:"sending_queue"`
}

type QueueSettings struct {
	// QueueSize set the length of the sending queue
	QueueSize int `mapstructure:"queue_size"`
}

var _ config.Exporter = (*Config)(nil)

// Validate config
func (config *Config) Validate() error {
	if config.LogGroupName == "" {
		return errors.New("'log_group_name' must be set")
	}
	if config.LogStreamName == "" {
		return errors.New("'log_stream_name' must be set")
	}
	if config.QueueSettings.QueueSize < 1 {
		return errors.New("'sending_queue.queue_size' must be 1 or greater")
	}
	return nil
}

func (config *Config) enforcedQueueSettings() exporterhelper.QueueSettings {
	return exporterhelper.QueueSettings{
		Enabled: true,
		// due to the sequence token, there can be only one request in flight
		NumConsumers: 1,
		QueueSize:    config.QueueSettings.QueueSize,
	}
}

// TODO(jbd): Add ARN role to config.
