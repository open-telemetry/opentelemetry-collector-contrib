// Copyright 2019, OpenTelemetry Authors
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

package awsxrayexporter

import "go.opentelemetry.io/collector/config/configmodels"

// Config defines configuration for AWS X-Ray exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	// Maximum number of concurrent calls to AWS X-Ray to upload documents.
	NumberOfWorkers int `mapstructure:"num_workers"`
	// X-Ray service endpoint to which the collector sends segment documents.
	Endpoint string `mapstructure:"endpoint"`
	// Number of seconds before timing out a request.
	RequestTimeoutSeconds int `mapstructure:"request_timeout_seconds"`
	// Maximum number of retries before abandoning an attempt to post data.
	MaxRetries int `mapstructure:"max_retries"`
	// Enable or disable TLS certificate verification.
	NoVerifySSL bool `mapstructure:"no_verify_ssl"`
	// Upload segments to AWS X-Ray through a proxy.
	ProxyAddress string `mapstructure:"proxy_address"`
	// Send segments to AWS X-Ray service in a specific region.
	Region string `mapstructure:"region"`
	// Local mode to skip EC2 instance metadata check.
	LocalMode bool `mapstructure:"local_mode"`
	// Amazon Resource Name (ARN) of the AWS resource running the collector.
	ResourceARN string `mapstructure:"resource_arn"`
	// IAM role to upload segments to a different account.
	RoleARN string `mapstructure:"role_arn"`
	// By default, OpenTelemetry attributes are converted to X-Ray metadata, which are not indexed.
	// Specify a list of attribute names to be converted to X-Ray annotations instead, which will be indexed.
	// See annotation vs. metadata: https://docs.aws.amazon.com/xray/latest/devguide/xray-concepts.html#xray-concepts-annotations
	IndexedAttributes []string `mapstructure:"indexed_attributes"`
	// Set to true to convert all OpenTelemetry attributes to X-Ray annotation (indexed) ignoring the IndexedAttributes option.
	// Default value: false
	IndexAllAttributes bool `mapstructure:"index_all_attributes"`
}
