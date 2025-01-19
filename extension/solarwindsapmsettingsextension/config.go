// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

import (
	"os"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
)

type Config struct {
	ClientConfig configgrpc.ClientConfig `mapstructure:",squash"`
	Key          string                  `mapstructure:"key"`
	Interval     time.Duration           `mapstructure:"interval"`
}

const (
	DefaultEndpoint = "apm.collector.na-01.cloud.solarwinds.com:443"
	DefaultInterval = time.Duration(10) * time.Second
	MinimumInterval = time.Duration(5) * time.Second
	MaximumInterval = time.Duration(60) * time.Second
)

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig: configgrpc.ClientConfig{
			Endpoint: DefaultEndpoint,
		},
		Interval: DefaultInterval,
	}
}

func (cfg *Config) Validate() error {
	// Endpoint
	matched, _ := regexp.MatchString(`apm.collector.[a-z]{2,3}-[0-9]{2}.[a-z\-]*.solarwinds.com:443`, cfg.ClientConfig.Endpoint)
	if !matched {
		// Replaced by the default
		cfg.ClientConfig.Endpoint = DefaultEndpoint
	}
	// Key
	keyArr := strings.Split(cfg.Key, ":")
	if len(keyArr) == 2 && len(keyArr[1]) == 0 {
		/**
		 * Service name is empty. We are trying our best effort to resolve the service name
		 */
		serviceName := resolveServiceNameBestEffort()
		if len(serviceName) > 0 {
			cfg.Key = keyArr[0] + ":" + serviceName
		}
	}
	// Interval
	if cfg.Interval.Seconds() < MinimumInterval.Seconds() {
		cfg.Interval = MinimumInterval
	}
	if cfg.Interval.Seconds() > MaximumInterval.Seconds() {
		cfg.Interval = MaximumInterval
	}
	return nil
}

func resolveServiceNameBestEffort() string {
	if otelServiceName, otelServiceNameDefined := os.LookupEnv("OTEL_SERVICE_NAME"); otelServiceNameDefined && len(otelServiceName) > 0 {
		return otelServiceName
	} else if awsLambdaFunctionName, awsLambdaFunctionNameDefined := os.LookupEnv("AWS_LAMBDA_FUNCTION_NAME"); awsLambdaFunctionNameDefined && len(awsLambdaFunctionName) > 0 {
		return awsLambdaFunctionName
	}
	return ""
}
