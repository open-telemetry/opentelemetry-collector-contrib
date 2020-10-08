// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"go.opentelemetry.io/collector/config/configmodels"
)

// Config defines configuration for AWS EMF exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	// LogGroupName is the name of CloudWatch log group which defines group of log streams
	// that share the same retention, monitoring, and access control settings.
	LogGroupName string `mapstructure:"log_group_name"`
	// LogStreamName is the name of CloudWatch log stream which is a sequence of log events
	// that share the same source.
	LogStreamName string `mapstructure:"log_stream_name"`
	// Namespace is a container for CloudWatch metrics.
	// Metrics in different namespaces are isolated from each other.
	Namespace string `mapstructure:"namespace"`
	// Endpoint is the CloudWatch Logs service endpoint which the requests
	// are forwarded to. https://docs.aws.amazon.com/general/latest/gr/cwl_region.html
	// e.g. logs.us-east-1.amazonaws.com and logs-fips.us-east-1.amazonaws.com
	Endpoint string `mapstructure:"endpoint"`
	// RequestTimeoutSeconds is number of seconds before a request times out.
	RequestTimeoutSeconds int `mapstructure:"request_timeout_seconds"`
	// ProxyAddress defines the proxy address that the local TCP server
	// forwards HTTP requests to AWS CloudWatch Logs backend through.
	ProxyAddress string `mapstructure:"proxy_address"`
	// Region is the AWS region where the metric logs are sent to.
	Region string `mapstructure:"region"`
	// RoleARN is the IAM role used by the collector when communicating
	// with the CloudWatch Logs service
	RoleARN string `mapstructure:"role_arn"`
	// NoVerifySSL is the option to disable TLS certificate verification.
	NoVerifySSL bool `mapstructure:"no_verify_ssl"`
	// MaxRetries is the maximum number of retries before abandoning an attempt to post data.
	MaxRetries int `mapstructure:"max_retries"`
	// DimensionRollupOption is the option for metrics dimension rollup. Three options are available, default option is "ZeroAndSingleDimensionRollup".
	// "ZeroAndSingleDimensionRollup" - Enable both zero dimension rollup and single dimension rollup
	// "SingleDimensionRollupOnly" - Enable single dimension rollup
	// "NoDimensionRollup" - No dimension rollup (only keep original metrics which contain all dimensions)
	DimensionRollupOption string `mapstructure:"dimension_rollup_option"`
}
