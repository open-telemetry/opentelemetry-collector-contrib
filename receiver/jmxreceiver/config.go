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

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Applications       map[string]ApplicationConfig `mapstructure:"applications"`
	OTLPExporterConfig otlpExporterConfig           `mapstructure:"otlp"`
}

type ApplicationConfig struct {
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
	Password configopaque.String `mapstructure:"password"`
	// The keystore path for SSL
	KeystorePath string `mapstructure:"keystore_path"`
	// The keystore password for SSL
	KeystorePassword configopaque.String `mapstructure:"keystore_password"`
	// The keystore type for SSL
	KeystoreType string `mapstructure:"keystore_type"`
	// The truststore path for SSL
	TruststorePath string `mapstructure:"truststore_path"`
	// The truststore password for SSL
	TruststorePassword configopaque.String `mapstructure:"truststore_password"`
	// The truststore type for SSL
	TruststoreType string `mapstructure:"truststore_type"`
	// The JMX remote profile.  Should be one of:
	// `"SASL/PLAIN"`, `"SASL/DIGEST-MD5"`, `"SASL/CRAM-MD5"`, `"TLS SASL/PLAIN"`, `"TLS SASL/DIGEST-MD5"`, or
	// `"TLS SASL/CRAM-MD5"`, though no enforcement is applied.
	RemoteProfile string `mapstructure:"remote_profile"`
	// The SASL/DIGEST-MD5 realm
	Realm string `mapstructure:"realm"`
	// Array of additional JARs to be added to the class path when launching the JMX Metric Gatherer JAR
	AdditionalJars []string `mapstructure:"additional_jars"`
	// Map of resource attributes used by the Java SDK Autoconfigure to set resource attributes
	ResourceAttributes map[string]string `mapstructure:"resource_attributes"`
	// Log level used by the JMX metric gatherer. Should be one of:
	// `"trace"`, `"debug"`, `"info"`, `"warn"`, `"error"`, `"off"`
	LogLevel string `mapstructure:"log_level"`
}

type otlpExporterConfig struct {
	Endpoint        string                       `mapstructure:"endpoint"`
	TimeoutSettings exporterhelper.TimeoutConfig `mapstructure:",squash"`
	Headers         map[string]string            `mapstructure:"headers"`
}

func (oec otlpExporterConfig) headersToString() string {
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
	headerString = headerString[0 : len(headerString)-1]
	return headerString
}

func (c *ApplicationConfig) parseProperties(logger *zap.Logger) []string {
	parsed := make([]string, 0, 1)

	logLevel := "info"
	if len(c.LogLevel) > 0 {
		logLevel = strings.ToLower(c.LogLevel)
	} else if logger != nil {
		logLevel = getZapLoggerLevelEquivalent(logger)
	}

	parsed = append(parsed, "-Dorg.slf4j.simpleLogger.defaultLogLevel="+logLevel)
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

func (c *ApplicationConfig) parseClasspath() string {
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

var (
	validLogLevels     = map[string]struct{}{"trace": {}, "debug": {}, "info": {}, "warn": {}, "error": {}, "off": {}}
	validTargetSystems = map[string]struct{}{
		"activemq": {}, "cassandra": {}, "hbase": {}, "hadoop": {},
		"jetty": {}, "jvm": {}, "kafka": {}, "kafka-consumer": {}, "kafka-producer": {}, "solr": {}, "tomcat": {}, "wildfly": {},
	}
)

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
	if len(c.Applications) == 0 {
		return fmt.Errorf("at least one application must be configured")
	}

	for appName, appConfig := range c.Applications {
		if appConfig.JARPath == "" {
			return fmt.Errorf("missing required field `jar_path` for application %s", appName)
		}
		if appConfig.Endpoint == "" {
			return fmt.Errorf("missing required field `endpoint` for application %s", appName)
		}
		if appConfig.TargetSystem == "" {
			return fmt.Errorf("missing required field `target_system` for application %s", appName)
		}
		if appConfig.CollectionInterval < 0 {
			return fmt.Errorf("`collection_interval` must be positive for application %s: %vms", appName, appConfig.CollectionInterval.Milliseconds())
		}
		if len(appConfig.LogLevel) > 0 {
			if _, ok := validLogLevels[strings.ToLower(appConfig.LogLevel)]; !ok {
				return fmt.Errorf("`log_level` must be one of %s for application %s", listKeys(validLogLevels), appName)
			}
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

func testLevel(logger *zap.Logger, level zapcore.Level) bool {
	return logger.Check(level, "_") != nil
}
