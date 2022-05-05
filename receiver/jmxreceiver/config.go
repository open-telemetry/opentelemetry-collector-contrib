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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
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
	// The truststore type for SSL
	TruststoreType string `mapstructure:"truststore_type"`
	// The JMX remote profile.  Should be one of:
	// `"SASL/PLAIN"`, `"SASL/DIGEST-MD5"`, `"SASL/CRAM-MD5"`, `"TLS SASL/PLAIN"`, `"TLS SASL/DIGEST-MD5"`, or
	// `"TLS SASL/CRAM-MD5"`, though no enforcement is applied.
	RemoteProfile string `mapstructure:"remote_profile"`
	// The SASL/DIGEST-MD5 realm
	Realm string `mapstructure:"realm"`
	// Map of property names to values to pass as system properties when running JMX Metric Gatherer
	Properties map[string]string `mapstructure:"properties"`
	// Array of additional JARs to be added to the the class path when launching the JMX Metric Gatherer JAR
	AdditionalJars []string `mapstructure:"additional_jars"`
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

// parseClasspath creates a classpath string with the JMX Gatherer JAR at the beginning
func (c *Config) parseClasspath() string {
	classPathElems := make([]string, 0)

	// See if the CLASSPATH env exists and if so get it's current value
	currentClassPath, ok := os.LookupEnv("CLASSPATH")
	if ok && currentClassPath != "" {
		classPathElems = append(classPathElems, currentClassPath)
	}

	// Add JMX JAR to classpath
	classPathElems = append(classPathElems, c.JARPath)

	// Add additional JARs if any
	classPathElems = append(classPathElems, c.AdditionalJars...)

	// Join them
	return strings.Join(classPathElems, ":")
}

type supportedJar struct {
	jar             string
	version         string
	addedValidation func(c *Config, j supportedJar) error
}

var jmxMetricsGathererVersions = map[string]supportedJar{
	"0646639df98404bd9b1263b46e2fd4612bc378f9951a561f0a0be9725718db36": {
		version:         "1.13.0",
		jar:             "JMX metrics gatherer",
		addedValidation: oldFormatProperties,
	},
	"c0b1a19c4965c7961abaaccfbb4d358e5f3b0b5b105578a4782702f126bfa8b7": {
		version:         "1.12.0",
		jar:             "JMX metrics gatherer",
		addedValidation: oldFormatProperties,
	},
	"ca689ca2da8a412c7f4ea0e816f47e8639b4270a48fb877c9a910b44757bc0a4": {
		version:         "1.11.0",
		jar:             "JMX metrics gatherer",
		addedValidation: oldFormatProperties,
	},
	"4b14d26fb383ed925fe1faf1b7fe2103559ed98ce6cf761ac9afc0158d2a218c": {
		version:         "1.10.0",
		jar:             "JMX metrics gatherer",
		addedValidation: oldFormatProperties,
	},
}

var additionalJarVersions = map[string]supportedJar{
	"hash": {
		version: "1.13.0",
		jar:     "JBoss Client",
	},
}

func oldFormatProperties(c *Config, j supportedJar) error {
	if c.KeystorePassword != "" ||
		c.KeystorePath != "" ||
		c.KeystoreType != "" ||
		c.TruststorePassword != "" ||
		c.TruststorePath != "" ||
		c.TruststoreType != "" {
		return fmt.Errorf("version %s of the JMX Metrics Gatherer does not support SSL parameters (Keystore & Truststore) "+
			"from the jmxreceiver. Update to the latest JMX Metrics Gatherer if you would like SSL support", j.version)
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (c *Config) validateJar(hashMap map[string]supportedJar, jar string) error {
	hash, err := hashFile(jar)
	if err != nil {
		return fmt.Errorf("error hashing file: %w", err)
	}

	jarDetails, ok := hashMap[hash]
	if !ok {
		return errors.New("jar hash does not match known versions")
	}
	if jarDetails.addedValidation != nil {
		if err = jarDetails.addedValidation(c, jarDetails); err != nil {
			return fmt.Errorf("jar failed validation: %w", err)
		}
	}

	return nil
}

func (c *Config) validate() error {
	var missingFields []string
	if c.JARPath == "" {
		missingFields = append(missingFields, "`jar_path`")
	}
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

	err := c.validateJar(jmxMetricsGathererVersions, c.JARPath)
	if err != nil {
		return fmt.Errorf("%v error validating `jar_path`: %w", c.ID(), err)
	}

	for _, additionalJar := range c.AdditionalJars {
		err := c.validateJar(additionalJarVersions, additionalJar)
		if err != nil {
			return fmt.Errorf("%v error validating `additional_jars`: %w", c.ID(), err)
		}
	}

	if c.CollectionInterval < 0 {
		return fmt.Errorf("%v `interval` must be positive: %vms", c.ID(), c.CollectionInterval.Milliseconds())
	}

	if c.OTLPExporterConfig.Timeout < 0 {
		return fmt.Errorf("%v `otlp.timeout` must be positive: %vms", c.ID(), c.OTLPExporterConfig.Timeout.Milliseconds())
	}

	return nil
}
