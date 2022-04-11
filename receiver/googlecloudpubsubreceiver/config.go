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

package googlecloudpubsubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"

import (
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

var subscriptionMatcher = regexp.MustCompile(`projects/[a-z][a-z0-9\-]*/subscriptions/`)

type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`

	// Google Cloud Project ID where the Pubsub client will connect to
	ProjectID string `mapstructure:"project"`
	// User agent that will be used by the Pubsub client to connect to the service
	UserAgent string `mapstructure:"user_agent"`
	// Override of the Pubsub endpoint, for testing only
	endpoint string
	// Only has effect if Endpoint is not ""
	insecure bool
	// Timeout for all API calls. If not set, defaults to 12 seconds.
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// The fully qualified resource name of the Pubsub subscription
	Subscription string `mapstructure:"subscription"`
	// Lock down the encoding of the payload, leave empty for attribute based detection
	Encoding string `mapstructure:"encoding"`
	// Lock down the compression of the payload, leave empty for attribute based detection
	Compression string `mapstructure:"compression"`

	// The client id that will be used by Pubsub to make load balancing decisions
	ClientID string `mapstructure:"client_id"`
}

func (config *Config) validateForLog() error {
	err := config.validate()
	if err != nil {
		return err
	}
	switch config.Encoding {
	case "":
	case "otlp_proto_log":
	case "raw_text":
	case "raw_json":
	default:
		return fmt.Errorf("log encoding %v is not supported.  supported encoding formats include [otlp_proto_log,raw_text,raw_json]", config.Encoding)
	}
	return nil
}

func (config *Config) validateForTrace() error {
	err := config.validate()
	if err != nil {
		return err
	}
	switch config.Encoding {
	case "":
	case "otlp_proto_trace":
	default:
		return fmt.Errorf("trace encoding %v is not supported.  supported encoding formats include [otlp_proto_trace]", config.Encoding)
	}
	return nil
}

func (config *Config) validateForMetric() error {
	err := config.validate()
	if err != nil {
		return err
	}
	switch config.Encoding {
	case "":
	case "otlp_proto_metric":
	default:
		return fmt.Errorf("metric encoding %v is not supported.  supported encoding formats include [otlp_proto_metric]", config.Encoding)
	}
	return nil
}

func (config *Config) validate() error {
	if !subscriptionMatcher.MatchString(config.Subscription) {
		return fmt.Errorf("subscription '%s' is not a valid format, use 'projects/<project_id>/subscriptions/<name>'", config.Subscription)
	}
	switch config.Compression {
	case "":
	case "gzip":
	default:
		return fmt.Errorf("compression %v is not supported.  supported compression formats include [gzip]", config.Compression)
	}
	return nil
}
