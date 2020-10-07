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
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
)

type config struct {
	configmodels.ExtensionSettings `mapstructure:",squash"`
	// The path for the JMX Metric Gatherer uber jar (/opt/opentelemetry-java-contrib-jmx-metrics.jar by default).
	JarPath string `mapstructure:"jar_path"`
	// The target JMX service url.
	ServiceURL string `mapstructure:"service_url"`
	// The target system for the metric gatherer whose built in groovy script to run.  Cannot be set with GroovyScript.
	TargetSystem string `mapstructure:"target_system"`
	// The script for the metric gatherer to run on the configured interval.  Cannot be set with TargetSystem.
	GroovyScript string `mapstructure:"groovy_script"`
	// The duration in between groovy script invocations and metric exports (10 seconds by default).
	// Will be converted to milliseconds.
	Interval time.Duration `mapstructure:"interval"`
	// The JMX username
	Username string `mapstructure:"username"`
	// The JMX password
	Password string `mapstructure:"password"`
	// The metric exporter to use ("otlp" or "prometheus", "otlp" by default).
	Exporter string `mapstructure:"exporter"`
	// The Otlp Receiver endpoint to send metrics to ("localhost:55680" by default).
	OtlpEndpoint string `mapstructure:"otlp_endpoint"`
	// The Otlp exporter timeout (5 seconds by default).  Will be converted to milliseconds.
	OtlpTimeout time.Duration `mapstructure:"otlp_timeout"`
	// The headers to include in otlp metric submission requests.
	OtlpHeaders map[string]string `mapstructure:"otlp_headers"`
	// The Prometheus Host
	PromethusHost string `mapstructure:"prometheus_host"`
	// The Prometheus Port
	PromethusPort int `mapstructure:"prometheus_port"`
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
	if c.TargetSystem == "" && c.GroovyScript == "" {
		missingFields = append(missingFields, "`target_system` or `groovy_script`")
	}
	if missingFields != nil {
		baseMsg := fmt.Sprintf("%v missing required field", c.Name())
		if len(missingFields) > 1 {
			baseMsg += "s"
		}
		return fmt.Errorf("%v: %v", baseMsg, strings.Join(missingFields, ", "))
	}

	if c.Interval < 0 {
		return fmt.Errorf("%v `interval` must be positive: %vms", c.Name(), c.Interval.Milliseconds())
	}

	if c.OtlpTimeout < 0 {
		return fmt.Errorf("%v `otlp_timeout` must be positive: %vms", c.Name(), c.OtlpTimeout.Milliseconds())
	}
	return nil
}
