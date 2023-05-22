// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
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
	// The truststore type for SSL
	TruststoreType string `mapstructure:"truststore_type"`
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

func (c *Config) parseProperties(logger *zap.Logger) []string {
	parsed := make([]string, 0, 1)

	logLevel := "info"
	if len(c.LogLevel) > 0 {
		logLevel = strings.ToLower(c.LogLevel)
	} else if logger != nil {
		logLevel = getZapLoggerLevelEquivalent(logger)
	}

	parsed = append(parsed, fmt.Sprintf("-Dorg.slf4j.simpleLogger.defaultLogLevel=%s", logLevel))
	// Sorted for testing and reproducibility
	sort.Strings(parsed)
	return parsed
}

var logLevelTranslator = map[zapcore.Level]string{
	zap.DebugLevel:  "debug",
	zap.InfoLevel:   "info",
	zap.WarnLevel:   "warn",
	zap.ErrorLevel:  "error",
	zap.DPanicLevel: "error",
	zap.PanicLevel:  "error",
	zap.FatalLevel:  "error",
}

var zapLevels = []zapcore.Level{
	zap.DebugLevel,
	zap.InfoLevel,
	zap.WarnLevel,
	zap.ErrorLevel,
	zap.DPanicLevel,
	zap.PanicLevel,
	zap.FatalLevel,
}

func getZapLoggerLevelEquivalent(logger *zap.Logger) string {
	var loggerLevel *zapcore.Level
	for i, level := range zapLevels {
		if testLevel(logger, level) {
			loggerLevel = &zapLevels[i]
			break
		}
	}

	// Couldn't get log level from logger default logger level to info
	if loggerLevel == nil {
		return "info"
	}

	return logLevelTranslator[*loggerLevel]
}

func testLevel(logger *zap.Logger, level zapcore.Level) bool {
	return logger.Check(level, "_") != nil
}

// parseClasspath creates a classpath string with the JMX Gatherer JAR at the beginning
func (c *Config) parseClasspath() string {
	var classPathElems []string

	// Add JMX JAR to classpath
	classPathElems = append(classPathElems, c.JARPath)

	// Add additional JARs if any
	classPathElems = append(classPathElems, c.AdditionalJars...)

	// Join them
	return strings.Join(classPathElems, ":")
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

func (c *Config) validateJar(supportedJarDetails map[string]supportedJar, jar string) error {
	hash, err := hashFile(jar)
	if err != nil {
		return fmt.Errorf("error hashing file: %w", err)
	}

	jarDetails, ok := supportedJarDetails[hash]
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

var validLogLevels = map[string]struct{}{"trace": {}, "debug": {}, "info": {}, "warn": {}, "error": {}, "off": {}}
var validTargetSystems = map[string]struct{}{"activemq": {}, "cassandra": {}, "hbase": {}, "hadoop": {},
	"jetty": {}, "jvm": {}, "kafka": {}, "kafka-consumer": {}, "kafka-producer": {}, "solr": {}, "tomcat": {}, "wildfly": {}}
var AdditionalTargetSystems = "n/a"

// Separated into two functions for tests
func init() {
	initAdditionalTargetSystems()
}

func initAdditionalTargetSystems() {
	if AdditionalTargetSystems != "n/a" {
		additionalTargets := strings.Split(AdditionalTargetSystems, ",")
		for _, t := range additionalTargets {
			validTargetSystems[t] = struct{}{}
		}
	}
}

func (c *Config) Validate() error {
	var missingFields []string
	if c.JARPath == "" {
		missingFields = append(missingFields, "`jar_path`")
	}
	if c.Endpoint == "" {
		missingFields = append(missingFields, "`endpoint`")
	}
	if c.TargetSystem == "" {
		missingFields = append(missingFields, "`target_system`")
	}
	if missingFields != nil {
		return fmt.Errorf("missing required field(s): %v", strings.Join(missingFields, ", "))
	}

	err := c.validateJar(jmxMetricsGathererVersions, c.JARPath)
	if err != nil {
		return fmt.Errorf("invalid `jar_path`: %w", err)
	}

	for _, additionalJar := range c.AdditionalJars {
		err := c.validateJar(wildflyJarVersions, additionalJar)
		if err != nil {
			return fmt.Errorf("invalid `additional_jars`. Additional Jar should be a jboss-client.jar from Wildfly, "+
				"no other integrations require additional jars at this time: %w", err)
		}
	}

	if c.CollectionInterval < 0 {
		return fmt.Errorf("`interval` must be positive: %vms", c.CollectionInterval.Milliseconds())
	}

	if c.OTLPExporterConfig.Timeout < 0 {
		return fmt.Errorf("`otlp.timeout` must be positive: %vms", c.OTLPExporterConfig.Timeout.Milliseconds())
	}

	if len(c.LogLevel) > 0 {
		if _, ok := validLogLevels[strings.ToLower(c.LogLevel)]; !ok {
			return fmt.Errorf("`log_level` must be one of %s", listKeys(validLogLevels))
		}
	}

	for _, system := range strings.Split(c.TargetSystem, ",") {
		if _, ok := validTargetSystems[strings.ToLower(system)]; !ok {
			return fmt.Errorf("`target_system` list may only be a subset of %s", listKeys(validTargetSystems))
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
