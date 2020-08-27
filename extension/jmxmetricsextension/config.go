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

package jmxmetricsextension

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
)

type config struct {
	configmodels.ExtensionSettings `mapstructure:",squash"`
	// The target JMX service url.
	ServiceURL string `mapstructure:"service_url"`
	// The script for the metric gatherer to run on the configured interval.
	GroovyScript string `mapstructure:"groovy_script"`
	// The duration in between groovy script invocations and metric exports.  Will be converted to milliseconds.
	Interval time.Duration `mapstructure:"interval"`
	// The JMX username
	Username string `mapstructure:"username"`
	// The JMX password
	Password string `mapstructure:"password"`
	// The otlp exporter timeout.  Will be converted to milliseconds.
	OtlpTimeout time.Duration `mapstructure:"otlp_timeout"`
	// The headers to include in otlp metric submission requests.
	OtlpHeaders map[string]string `mapstructure:"otlp_headers"`
	// The keystore path for SSL
	KeystorePath string `mapstructure:"keystore_path"`
	// The keystore password for SSL
	KeystorePassword string `mapstructure:"keystore_password"`
	// The keystore type for SSL
	KeystoreType string `mapstructure:"keystore_type"`
	// The truststore path for SSL
	TruststorePath string `mapstructure:"truststore_path"`
	// The truststore password for SSL
	TruststorePassword string `mapstructure:"truststore_password"`
	// The JMX remote profile.  Should be one of:
	// `"SASL/PLAIN"`, `"SASL/DIGEST-MD5"`, `"SASL/CRAM-MD5"`, `"TLS SASL/PLAIN"`, `"TLS SASL/DIGEST-MD5"`, or
	// `"TLS SASL/CRAM-MD5"`, though no enforcement is applied.
	RemoteProfile string `mapstructure:"remote_profile"`
	// The SASL/DIGEST-MD5 realm
	Realm string `mapstructure:"realm"`
}

func (c *config) validate() error {
	var missingFields []string
	if c.ServiceURL == "" {
		missingFields = append(missingFields, "`service_url`")
	}
	if c.GroovyScript == "" {
		missingFields = append(missingFields, "`groovy_script`")
	}
	if missingFields != nil {
		baseMsg := fmt.Sprintf("%v missing required field", c.Name())
		if len(missingFields) > 1 {
			baseMsg += "s"
		}
		return fmt.Errorf("%v: %v", baseMsg, missingFields)
	}

	if c.Interval < 0 {
		return fmt.Errorf("%v `interval` must be positive: %vms", c.Name(), c.Interval.Milliseconds())
	}

	if c.OtlpTimeout < 0 {
		return fmt.Errorf("%v `otlp_timeout` must be positive: %vms", c.Name(), c.OtlpTimeout.Milliseconds())
	}
	return nil
}
