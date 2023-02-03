// Copyright The OpenTelemetry Authors
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

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	// HTTPServerSettings is used to set up the Firehose delivery
	// endpoint. The Firehose delivery stream expects an HTTPS
	// endpoint, so TLSSettings must be used to enable that.
	confighttp.HTTPServerSettings `mapstructure:",squash"`
	// RecordType is the key used to determine which unmarshaler to use
	// when receiving the requests.
	RecordType string `mapstructure:"record_type"`
	// AccessKey is checked against the one received with each request.
	// This can be set when creating or updating the Firehose delivery
	// stream.
	AccessKey string `mapstructure:"access_key"`
}

// Validate checks that the endpoint and record type exist and
// are valid.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("must specify endpoint")
	}
	if c.RecordType == "" {
		return errors.New("must specify record type")
	}
	if err := validateRecordType(c.RecordType); err != nil {
		return err
	}
	return nil
}
