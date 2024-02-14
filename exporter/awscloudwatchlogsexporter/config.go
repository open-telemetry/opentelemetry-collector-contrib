// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"
)

// Config represent a configuration for the CloudWatch logs exporter.
type Config struct {
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	// LogGroupName is the name of CloudWatch log group which defines group of log streams
	// that share the same retention, monitoring, and access control settings.
	LogGroupName string `mapstructure:"log_group_name"`

	// LogStreamName is the name of CloudWatch log stream which is a sequence of log events
	// that share the same source.
	LogStreamName string `mapstructure:"log_stream_name"`

	// Endpoint is the CloudWatch Logs service endpoint which the requests
	// are forwarded to. https://docs.aws.amazon.com/general/latest/gr/cwl_region.html
	// e.g. logs.us-east-1.amazonaws.com
	// Optional.
	Endpoint string `mapstructure:"endpoint"`

	// LogRetention is the option to set the log retention policy for the CloudWatch Log Group. Defaults to Never Expire if not specified or set to 0
	// Possible values are 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 2192, 2557, 2922, 3288, or 3653
	LogRetention int64 `mapstructure:"log_retention"`

	// Tags is the option to set tags for the CloudWatch Log Group.  If specified, please add add at least 1 and at most 50 tags.  Input is a string to string map like so: { 'key': 'value' }
	// Keys must be between 1-128 characters and follow the regex pattern: ^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$
	// Values must be between 1-256 characters and follow the regex pattern: ^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$
	Tags map[string]*string `mapstructure:"tags"`

	// Queue settings frm the exporterhelper
	exporterhelper.QueueSettings `mapstructure:"sending_queue"`

	logger *zap.Logger

	awsutil.AWSSessionSettings `mapstructure:",squash"`

	// Export raw log string instead of log wrapper
	// Required for emf logs
	RawLog bool `mapstructure:"raw_log,omitempty"`
}

var _ component.Config = (*Config)(nil)

// Validate config
func (config *Config) Validate() error {
	if config.LogGroupName == "" {
		return errors.New("'log_group_name' must be set")
	}
	if config.LogStreamName == "" {
		return errors.New("'log_stream_name' must be set")
	}

	if err := config.QueueSettings.Validate(); err != nil {
		return err
	}

	if retErr := cwlogs.ValidateRetentionValue(config.LogRetention); retErr != nil {
		return retErr
	}
	return cwlogs.ValidateTagsInput(config.Tags)

}

// TODO(jbd): Add ARN role to config.
