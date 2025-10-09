// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// jmxGathererMainClass the class containing the main function for the JMX Metric Gatherer JAR
var jmxGathererMainClass = "io.opentelemetry.contrib.jmxmetrics.JmxMetrics"

// jmxScraperMainClass the class containing the main function for the JMX Scraper JAR
var jmxScraperMainClass = "io.opentelemetry.contrib.jmxscraper.JmxScraper"

type Config struct {
	// The path for the JMX Metric Gatherer or JMX Scraper JAR (/opt/opentelemetry-java-contrib-jmx-metrics.jar by default).
	// Supported by: jmx-scraper and jmx-metric-gatherer
	JARPath string `mapstructure:"jar_path"`
	// The Service URL or host:port for the target coerced to one of form: service:jmx:rmi:///jndi/rmi://<host>:<port>/jmxrmi.
	// Supported by: jmx-scraper and jmx-metric-gatherer
	Endpoint string `mapstructure:"endpoint"`
	// Comma-separated list of systems to monitor
	// Supported by: jmx-scraper and jmx-metric-gatherer
	TargetSystem string `mapstructure:"target_system"`
	// The target source of metric definitions to use for the target system.
	// Supported values are: auto, instrumentation and legacy.
	// Supported by: jmx-scraper
	TargetSource string `mapstructure:"target_source"`
	// Comma-separated list of paths to custom YAML metrics definition,
	// mandatory when TargetSystem is not set.
	// Supported by: jmx-scraper
	JmxConfigs string `mapstructure:"jmx_configs"`
	// The duration in between groovy script invocations and metric exports (10 seconds by default).
	// Will be converted to milliseconds.
	// Supported by: jmx-scraper and jmx-metric-gatherer
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
	// The OTLP exporter settings
	// Supported by: jmx-scraper and jmx-metric-gatherer
	OTLPExporterConfig otlpExporterConfig `mapstructure:"otlp"`
	// The JMX username
	// Supported by: jmx-scraper and jmx-metric-gatherer
	Username string `mapstructure:"username"`
	// The JMX password
	// Supported by: jmx-scraper and jmx-metric-gatherer
	Password configopaque.String `mapstructure:"password"`
	// The keystore path for SSL
	// Supported by: jmx-scraper and jmx-metric-gatherer
	KeystorePath string `mapstructure:"keystore_path"`
	// The keystore password for SSL
	// Supported by: jmx-scraper and jmx-metric-gatherer
	KeystorePassword configopaque.String `mapstructure:"keystore_password"`
	// The keystore type for SSL
	// Supported by: jmx-scraper and jmx-metric-gatherer
	KeystoreType string `mapstructure:"keystore_type"`
	// The truststore path for SSL
	// Supported by: jmx-scraper and jmx-metric-gatherer
	TruststorePath string `mapstructure:"truststore_path"`
	// The truststore password for SSL
	// Supported by: jmx-scraper and jmx-metric-gatherer
	TruststorePassword configopaque.String `mapstructure:"truststore_password"`
	// The truststore type for SSL
	// Supported by: jmx-scraper and jmx-metric-gatherer
	TruststoreType string `mapstructure:"truststore_type"`
	// The JMX remote profile.  Should be one of:
	// `"SASL/PLAIN"`, `"SASL/DIGEST-MD5"`, `"SASL/CRAM-MD5"`, `"TLS SASL/PLAIN"`, `"TLS SASL/DIGEST-MD5"`, or
	// `"TLS SASL/CRAM-MD5"`, though no enforcement is applied.
	// Supported by: jmx-scraper and jmx-metric-gatherer
	RemoteProfile string `mapstructure:"remote_profile"`
	// The SASL/DIGEST-MD5 realm
	// Supported by: jmx-scraper and jmx-metric-gatherer
	Realm string `mapstructure:"realm"`
	// Array of additional JARs to be added to the class path when launching the JMX Metric Gatherer JAR
	// Supported by: jmx-scraper and jmx-metric-gatherer
	AdditionalJars []string `mapstructure:"additional_jars"`
	// Map of resource attributes used by the Java SDK Autoconfigure to set resource attributes
	// Supported by: jmx-scraper and jmx-metric-gatherer
	ResourceAttributes map[string]string `mapstructure:"resource_attributes"`
	// Log level used by the JMX metric gatherer. Should be one of:
	// `"trace"`, `"debug"`, `"info"`, `"warn"`, `"error"`, `"off"`
	// Supported by: jmx-metric-gatherer
	LogLevel string `mapstructure:"log_level"`
}

// We don't embed the existing OTLP Exporter config as most fields are unsupported
type otlpExporterConfig struct {
	// The OTLP Receiver endpoint to send metrics to ("0.0.0.0:<random open port>" by default).
	Endpoint string `mapstructure:"endpoint"`
	// The OTLP exporter timeout (5 seconds by default).  Will be converted to milliseconds.
	TimeoutSettings exporterhelper.TimeoutConfig `mapstructure:",squash"`
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
	// slf4j.simpleLogger only available in JMX Metrics Gatherer jar
	if err := c.validateJar(jmxMetricsGathererVersions, c.JARPath); err == nil {
		parsed := make([]string, 0, 1)

		logLevel := "info"
		if c.LogLevel != "" {
			logLevel = strings.ToLower(c.LogLevel)
		} else if logger != nil {
			logLevel = getZapLoggerLevelEquivalent(logger)
		}

		parsed = append(parsed, "-Dorg.slf4j.simpleLogger.defaultLogLevel="+logLevel)
		// Sorted for testing and reproducibility
		sort.Strings(parsed)
		return parsed
	}
	return nil
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

func isSupportedJAR(supportedJarDetails map[string]supportedJar, jar string) bool {
	hash, err := hashFile(jar)
	if err != nil {
		return false
	}
	_, ok := supportedJarDetails[hash]
	return ok
}

func (c *Config) jarMainClass() string {
	if isSupportedJAR(jmxMetricsGathererVersions, c.JARPath) {
		return jmxGathererMainClass
	} else if isSupportedJAR(jmxScraperVersions, c.JARPath) {
		return jmxScraperMainClass
	}
	return ""
}

func (c *Config) jarJMXSamplingConfig() (string, string) {
	if isSupportedJAR(jmxMetricsGathererVersions, c.JARPath) {
		return "otel.jmx.interval.milliseconds", strconv.FormatInt(c.CollectionInterval.Milliseconds(), 10)
	} else if isSupportedJAR(jmxScraperVersions, c.JARPath) {
		return "otel.metric.export.interval", c.CollectionInterval.String()
	}
	return "", ""
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
	validLogLevels = map[string]struct{}{"trace": {}, "debug": {}, "info": {}, "warn": {}, "error": {}, "off": {}}
	// feature parity between jmx-gatherer and jmx-scraper
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
		for t := range strings.SplitSeq(AdditionalTargetSystems, ",") {
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
		// jmx-scraper can use jmx_configs instead
		if c.validateJar(jmxScraperVersions, c.JARPath) == nil {
			if c.JmxConfigs == "" {
				missingFields = append(missingFields, "`target_system`", "`jmx_configs`")
			}
		} else {
			missingFields = append(missingFields, "`target_system`")
		}
	}
	if missingFields != nil {
		return fmt.Errorf("missing required field(s): %v", strings.Join(missingFields, ", "))
	}

	jmxScraperErr := c.validateJar(jmxScraperVersions, c.JARPath)
	jmxGathererErr := c.validateJar(jmxMetricsGathererVersions, c.JARPath)
	if jmxScraperErr != nil && jmxGathererErr != nil {
		return fmt.Errorf("invalid `jar_path`: %w", jmxScraperErr)
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

	if c.OTLPExporterConfig.TimeoutSettings.Timeout < 0 {
		return fmt.Errorf("`otlp.timeout` must be positive: %vms", c.OTLPExporterConfig.TimeoutSettings.Timeout.Milliseconds())
	}

	if c.LogLevel != "" {
		if isSupportedJAR(jmxScraperVersions, c.JARPath) {
			return errors.New("`log_level` can only be used with a JMX Metrics Gatherer JAR")
		}
		if _, ok := validLogLevels[strings.ToLower(c.LogLevel)]; !ok {
			return fmt.Errorf("`log_level` must be one of %s", listKeys(validLogLevels))
		}
	}

	if c.TargetSystem != "" {
		for system := range strings.SplitSeq(c.TargetSystem, ",") {
			if _, ok := validTargetSystems[strings.ToLower(system)]; !ok {
				return fmt.Errorf("`target_system` list may only be a subset of %s", listKeys(validTargetSystems))
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

func (c *Config) buildJMXConfig() (string, error) {
	config := map[string]string{}
	failedToParse := `failed to parse Endpoint "%s": %w`
	parsed, err := url.Parse(c.Endpoint)
	if err != nil {
		return "", fmt.Errorf(failedToParse, c.Endpoint, err)
	}

	if parsed.Scheme != "service" || !strings.HasPrefix(parsed.Opaque, "jmx:") {
		host, portStr, err := net.SplitHostPort(c.Endpoint)
		if err != nil {
			return "", fmt.Errorf(failedToParse, c.Endpoint, err)
		}
		port, err := strconv.ParseInt(portStr, 10, 0)
		if err != nil {
			return "", fmt.Errorf(failedToParse, c.Endpoint, err)
		}
		c.Endpoint = fmt.Sprintf("service:jmx:rmi:///jndi/rmi://%v:%d/jmxrmi", host, port)
	}

	config["otel.jmx.service.url"] = c.Endpoint
	samplingKey, samplingValue := c.jarJMXSamplingConfig()
	config[samplingKey] = samplingValue
	config["otel.jmx.target.system"] = c.TargetSystem

	endpoint := c.OTLPExporterConfig.Endpoint
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = "http://" + endpoint
	}

	config["otel.metrics.exporter"] = "otlp"
	config["otel.exporter.otlp.endpoint"] = endpoint
	config["otel.exporter.otlp.timeout"] = strconv.FormatInt(c.OTLPExporterConfig.TimeoutSettings.Timeout.Milliseconds(), 10)

	if len(c.OTLPExporterConfig.Headers) > 0 {
		config["otel.exporter.otlp.headers"] = c.OTLPExporterConfig.headersToString()
	}

	if c.Username != "" {
		config["otel.jmx.username"] = c.Username
	}

	if c.Password != "" {
		config["otel.jmx.password"] = string(c.Password)
	}

	if c.RemoteProfile != "" {
		config["otel.jmx.remote.profile"] = c.RemoteProfile
	}

	if c.Realm != "" {
		config["otel.jmx.realm"] = c.Realm
	}

	if c.KeystorePath != "" {
		config["javax.net.ssl.keyStore"] = c.KeystorePath
	}
	if c.KeystorePassword != "" {
		config["javax.net.ssl.keyStorePassword"] = string(c.KeystorePassword)
	}
	if c.KeystoreType != "" {
		config["javax.net.ssl.keyStoreType"] = c.KeystoreType
	}
	if c.TruststorePath != "" {
		config["javax.net.ssl.trustStore"] = c.TruststorePath
	}
	if c.TruststorePassword != "" {
		config["javax.net.ssl.trustStorePassword"] = string(c.TruststorePassword)
	}
	if c.TruststoreType != "" {
		config["javax.net.ssl.trustStoreType"] = c.TruststoreType
	}

	if len(c.ResourceAttributes) > 0 {
		attributes := make([]string, 0, len(c.ResourceAttributes))
		for k, v := range c.ResourceAttributes {
			attributes = append(attributes, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(attributes)
		config["otel.resource.attributes"] = strings.Join(attributes, ",")
	}

	// set jmx-scraper specific config options
	if isSupportedJAR(jmxScraperVersions, c.JARPath) {
		// jmx-scraper default target source: https://github.com/open-telemetry/opentelemetry-java-contrib/tree/main/jmx-scraper#configuration-reference
		if c.TargetSource != "" {
			config["otel.jmx.target.source"] = c.TargetSource
		} else {
			config["otel.jmx.target.source"] = "auto"
		}
		if c.JmxConfigs != "" {
			config["otel.jmx.config"] = c.JmxConfigs
		}
	}

	content := make([]string, 0, len(config))
	for k, v := range config {
		// Documentation of Java Properties format & escapes: https://docs.oracle.com/javase/7/docs/api/java/util/Properties.html#load(java.io.Reader)

		// Keys are receiver-defined so this escape should be unnecessary but in case that assumption
		// breaks in the future this will ensure keys are properly escaped
		safeKey := strings.ReplaceAll(k, "=", "\\=")
		safeKey = strings.ReplaceAll(safeKey, ":", "\\:")
		// Any whitespace must be removed from keys
		safeKey = strings.ReplaceAll(safeKey, " ", "")
		safeKey = strings.ReplaceAll(safeKey, "\t", "")
		safeKey = strings.ReplaceAll(safeKey, "\n", "")

		// Unneeded escape tokens will be removed by the properties file loader, so it should be pre-escaped to ensure
		// the values provided reach the metrics gatherer as provided. Also in case a user attempts to provide multiline
		// values for one of the available fields, we need to escape the newlines
		safeValue := strings.ReplaceAll(v, "\\", "\\\\")
		safeValue = strings.ReplaceAll(safeValue, "\n", "\\n")
		content = append(content, fmt.Sprintf("%s = %s", safeKey, safeValue))
	}
	sort.Strings(content)

	return strings.Join(content, "\n"), nil
}
