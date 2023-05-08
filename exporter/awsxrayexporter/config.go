// Copyright The OpenTelemetry Authors
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

package awsxrayexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
)

// Config defines configuration for AWS X-Ray exporter.
type Config struct {
	// AWSSessionSettings contains the common configuration options
	// for creating AWS session to communicate with backend
	awsutil.AWSSessionSettings `mapstructure:",squash"`
	// By default, OpenTelemetry attributes are converted to X-Ray metadata, which are not indexed.
	// Specify a list of attribute names to be converted to X-Ray annotations instead, which will be indexed.
	// See annotation vs. metadata: https://docs.aws.amazon.com/xray/latest/devguide/xray-concepts.html#xray-concepts-annotations
	IndexedAttributes []string `mapstructure:"indexed_attributes"`
	// Set to true to convert all OpenTelemetry attributes to X-Ray annotation (indexed) ignoring the IndexedAttributes option.
	// Default value: false
	IndexAllAttributes bool `mapstructure:"index_all_attributes"`

	LogGroupNames []string `mapstructure:"aws_log_groups"`
	// TelemetryConfig contains the options for telemetry collection.
	TelemetryConfig telemetry.Config `mapstructure:"telemetry,omitempty"`
}
