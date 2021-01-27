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

package otlp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/collector/config/confighttp"
)

// HTTPClientConfig makes confighttp.HTTPClientSettings marshallable with json and yaml
type HTTPClientConfig struct {
	confighttp.HTTPClientSettings
}

// NewHTTPClientConfig creates a new default config
func NewHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		confighttp.HTTPClientSettings{
			Endpoint: "https://localhost:55681/v1/logs",
		},
	}
}

// UnmarshalJSON will unmarshal json into a HTTPClientConfig struct
func (c *HTTPClientConfig) UnmarshalJSON(data []byte) error {
	any := make(map[string]interface{})
	if err := json.Unmarshal(data, &any); err != nil {
		return err
	}

	settings := confighttp.HTTPClientSettings{}
	if err := mapstructure.Decode(any, &settings); err != nil {
		return err
	}
	c.HTTPClientSettings = settings
	return nil
}

// UnmarshalYAML will unmarshal json into a HTTPClientConfig struct
func (c *HTTPClientConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var any interface{}
	if err := unmarshal(&any); err != nil {
		return err
	}

	settings := confighttp.HTTPClientSettings{}
	if err := mapstructure.Decode(any, &settings); err != nil {
		return err
	}
	c.HTTPClientSettings = settings
	return nil
}

func (c *HTTPClientConfig) cleanEndpoint() error {
	if c.Endpoint == "" {
		return fmt.Errorf("'endpoint' is required")
	}

	if !strings.HasPrefix(c.Endpoint, "http://") && !strings.HasPrefix(c.Endpoint, "https://") {
		if c.TLSSetting.Insecure {
			c.Endpoint = fmt.Sprintf("http://%s", c.Endpoint)
		} else {
			c.Endpoint = fmt.Sprintf("https://%s", c.Endpoint)
		}
	}

	if !strings.HasSuffix(c.Endpoint, "/v1/logs") {
		c.Endpoint = fmt.Sprintf("%s/v1/logs", c.Endpoint)
	}

	return nil
}
