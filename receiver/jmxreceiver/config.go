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

package jmxreceiver

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	// The path for the JMX Metric Gatherer uber JAR (/opt/opentelemetry-java-contrib-jmx-metrics.jar by default).
	JARPath string `mapstructure:"jar_path"`
	// The Service URL or host:port for the target coerced to one of form: service:jmx:rmi:///jndi/rmi://<host>:<port>/jmxrmi.
	Endpoint string `mapstructure:"endpoint"`
	// The target system for the metric gatherer whose built in groovy script to run.  Cannot be set with GroovyScript.
	TargetSystem string `mapstructure:"target_system"`
	// The script for the metric gatherer to run on the configured interval.  Cannot be set with TargetSystem.
	GroovyScript string `mapstructure:"groovy_script"`
	// The duration in between groovy script invocations and metric exports (10 seconds by default).
	// Will be converted to milliseconds.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
	// The exporter settings for
	OTLPExporterConfig otlpExporterConfig `mapstructure:"otlp"`
	// The JMX username
	Username string `mapstructure:"username"`
	// The JMX password
	Password string `mapstructure:"password"`
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
	// Map of property names to values to pass as system properties when running JMX Metric Gatherer
	Properties map[string]string `mapstructure:"properties"`
}

// We don't embed the existing OTLP Exporter config as most fields are unsupported
type otlpExporterConfig struct {
	// The OTLP Receiver endpoint to send metrics to ("0.0.0.0:<random open port>" by default).
	Endpoint string `mapstructure:"endpoint"`
	// The OTLP exporter timeout (5 seconds by default).  Will be converted to milliseconds.
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	// The headers to include in OTLP metric submission requests.
	Headers map[string]string `mapstructure:"headers"`
}

func (oec otlpExporterConfig) headersToString() string {
	// sort for reliable testing
	headers := make([]string, 0, len(oec.Headers))
	for k := range oec.Headers {
		headers = append(headers, k)
	}
	sort.Strings(headers)

	headerString := ""
	for _, k := range headers {
		v := oec.Headers[k]
		headerString += fmt.Sprintf("%s=%v,", k, v)
	}
	// remove trailing comma
	headerString = headerString[0 : len(headerString)-1]
	return headerString
}

func (c *Config) parseProperties() []string {
	parsed := make([]string, 0, len(c.Properties))
	for property, value := range c.Properties {
		parsed = append(parsed, fmt.Sprintf("-D%s=%s", property, value))
	}
	// Sorted for testing and reproducibility
	sort.Strings(parsed)
	return parsed
}

func (c *Config) validate() error {
	var missingFields []string
	if c.Endpoint == "" {
		missingFields = append(missingFields, "`endpoint`")
	}
	if c.TargetSystem == "" && c.GroovyScript == "" {
		missingFields = append(missingFields, "`target_system` or `groovy_script`")
	}
	if missingFields != nil {
		baseMsg := fmt.Sprintf("%v missing required field", c.ID())
		if len(missingFields) > 1 {
			baseMsg += "s"
		}
		return fmt.Errorf("%v: %v", baseMsg, strings.Join(missingFields, ", "))
	}

	if c.CollectionInterval < 0 {
		return fmt.Errorf("%v `interval` must be positive: %vms", c.ID(), c.CollectionInterval.Milliseconds())
	}

	if c.OTLPExporterConfig.Timeout < 0 {
		return fmt.Errorf("%v `otlp.timeout` must be positive: %vms", c.ID(), c.OTLPExporterConfig.Timeout.Milliseconds())
	}

	return nil
}
