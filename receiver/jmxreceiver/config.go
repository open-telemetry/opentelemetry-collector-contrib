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

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

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
	// The target system for the metric gatherer whose built in groovy script to run.
	TargetSystem string `mapstructure:"target_system"`
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
	// Array of additional JARs to be added to the the class path when launching the JMX Metric Gatherer JAR
	AdditionalJars []string `mapstructure:"additional_jars"`
	// Map of resource attributes used by the Java SDK Autoconfigure to set resource attributes
	ResourceAttributes map[string]string `mapstructure:"resource_attributes"`
	// Log level used by the JMX metric gatherer. Should be one of:
	// `"trace"`, `"debug"`, `"info"`, `"warn"`, `"error"`, `"off"`
	LogLevel string `mapstructure:"log_level"`
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
	parsed := make([]string, 0, 1)
	logLevel := "info"
	if len(c.LogLevel) > 0 {
		logLevel = strings.ToLower(c.LogLevel)
	}

	parsed = append(parsed, fmt.Sprintf("-Dorg.slf4j.simpleLogger.defaultLogLevel=%s", logLevel))
	// Sorted for testing and reproducibility
	sort.Strings(parsed)
	return parsed
}

// parseClasspath creates a classpath string with the JMX Gatherer JAR at the beginning
func (c *Config) parseClasspath() string {
	classPathElems := make([]string, 0)

	// Add JMX JAR to classpath
	classPathElems = append(classPathElems, c.JARPath)

	// Add additional JARs if any
	classPathElems = append(classPathElems, c.AdditionalJars...)

	// Join them
	return strings.Join(classPathElems, ":")
}

var validLogLevels = map[string]struct{}{"trace": {}, "debug": {}, "info": {}, "warn": {}, "error": {}, "off": {}}
var validTargetSystems = map[string]struct{}{"activemq": {}, "cassandra": {}, "hbase": {}, "hadoop": {},
	"jvm": {}, "kafka": {}, "kafka-consumer": {}, "kafka-producer": {}, "solr": {}, "tomcat": {}, "wildfly": {}}

func (c *Config) validate() error {
	var missingFields []string
	if c.Endpoint == "" {
		missingFields = append(missingFields, "`endpoint`")
	}
	if c.TargetSystem == "" {
		missingFields = append(missingFields, "`target_system`")
	}
	if c.JARPath == "" {
		missingFields = append(missingFields, "`jar_path`")
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

	if _, ok := validLogLevels[strings.ToLower(c.LogLevel)]; !ok {
		return fmt.Errorf("%v `log_level` must be one of %s", c.ID(), listKeys(validLogLevels))
	}

	for _, system := range strings.Split(c.TargetSystem, ",") {
		if _, ok := validTargetSystems[strings.ToLower(system)]; !ok {
			return fmt.Errorf("%v `target_system` list may only be a subset of %s", c.ID(), listKeys(validTargetSystems))
		}
	}

	return nil
}

func listKeys(presenceMap map[string]struct{}) string {
	list := make([]string, 0, len(presenceMap))
	for k := range presenceMap {
		list = append(list, fmt.Sprintf("'%s'", k))
	}
	sort.Strings(list)
	return strings.Join(list, ", ")
}
