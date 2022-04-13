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

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// oldDefaultConfig returns the old default config we used to have before v0.50.
func oldDefaultConfig() *Config {
	// This used to be done at 'GetHostTags', done here now
	var tags []string
	if os.Getenv("DD_TAGS") != "" {
		tags = strings.Split(os.Getenv("DD_TAGS"), " ")
	}

	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("datadog")),
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: 15 * time.Second,
		},
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),
		API: APIConfig{
			Key:  os.Getenv("DD_API_KEY"), // Must be set if using API
			Site: os.Getenv("DD_SITE"),    // If not provided, set during config sanitization
		},

		TagsConfig: TagsConfig{
			Hostname: os.Getenv("DD_HOST"),
			Env:      os.Getenv("DD_ENV"),
			Service:  os.Getenv("DD_SERVICE"),
			Version:  os.Getenv("DD_VERSION"),
			Tags:     tags,
		},

		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: os.Getenv("DD_URL"), // If not provided, set during config sanitization
			},
			SendMonotonic: true,
			DeltaTTL:      3600,
			Quantiles:     true,
			ExporterConfig: MetricsExporterConfig{
				ResourceAttributesAsTags:             false,
				InstrumentationLibraryMetadataAsTags: false,
			},
			HistConfig: HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode: CumulativeMonotonicSumModeToDelta,
			},
		},
		Traces: TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: os.Getenv("DD_APM_URL"), // If not provided, set during config sanitization
			},
			IgnoreResources: []string{},
		},
		HostMetadata: HostMetadataConfig{
			Tags:           tags,
			Enabled:        true,
			HostnameSource: HostnameSourceFirstResource,
		},
		SendMetadata:        true,
		UseResourceMetadata: true,
	}
}

// errUsedEnvVar constructs an error instructing users not to rely on
// the automatic environment variable detection and set the environment variable
// explicitly on configuration instead.
func errUsedEnvVar(settingName, envVarName string) error {
	return fmt.Errorf(
		"%q env var is set but not used explicitly on %q. %q does not default to %q's value since v0.50.0",
		envVarName,
		settingName,
		settingName,
		envVarName,
	)
}

// tagsDiffer checks if the host tags from each configuration are different.
func tagsDiffer(cfgTags []string, oldTags []string) bool {
	if len(cfgTags) != len(oldTags) {
		return true
	}

	oldCfgSet := map[string]struct{}{}
	for _, tag := range cfgTags {
		oldCfgSet[tag] = struct{}{}
	}

	for _, tag := range cfgTags {
		if _, ok := oldCfgSet[tag]; !ok {
			// tag is in new but not old
			return true
		}
	}

	return false
}

// warnUseOfEnvVars returns warnings related to automatic environment variable detection.
// On v0.50.0 we removed automatic environment variable detection, we warn users if the
// pre-v0.50.0 configuration would have different values.
func warnUseOfEnvVars(configMap *config.Map, cfg *Config) (warnings []error) {
	// We don't see the raw YAML contents from our exporter so, to compare with the old default,
	// we unmarshal the config map on the old default config and compare the two structs.
	// Any differences will be due to the change in default configuration.
	oldCfg := oldDefaultConfig()
	err := configMap.UnmarshalExact(oldCfg)

	errIssue := fmt.Errorf("see github.com/open-telemetry/opentelemetry-collector-contrib/issues/8396 for more details")

	if err != nil {
		// This should never happen, since unmarshaling passed with the original config.
		// If it happens, report the error and instruct users to check the issue.
		return []error{
			fmt.Errorf("failed to determine usage of env variable defaults: %w", err),
			errIssue,
		}
	}

	// We could probably do this with reflection but I don't want to risk
	// an accidental panic so we check each field manually.
	if cfg.API.Key != oldCfg.API.Key {
		warnings = append(warnings, errUsedEnvVar("api.key", "DD_API_KEY"))
	}
	if cfg.API.Site != oldCfg.API.Site {
		warnings = append(warnings, errUsedEnvVar("api.site", "DD_SITE"))
	}
	if cfg.Hostname != oldCfg.Hostname {
		warnings = append(warnings, errUsedEnvVar("hostname", "DD_HOST"))
	}
	if cfg.Env != oldCfg.Env {
		warnings = append(warnings, errUsedEnvVar("env", "DD_ENV"))
	}
	if cfg.Service != oldCfg.Service {
		warnings = append(warnings, errUsedEnvVar("service", "DD_SERVICE"))
	}
	if cfg.Version != oldCfg.Version {
		warnings = append(warnings, errUsedEnvVar("version", "DD_VERSION"))
	}
	if cfg.Metrics.Endpoint != oldCfg.Metrics.Endpoint {
		warnings = append(warnings, errUsedEnvVar("metrics.endpoint", "DD_URL"))
	}
	if cfg.Traces.Endpoint != oldCfg.Traces.Endpoint {
		warnings = append(warnings, errUsedEnvVar("traces.endpoint", "DD_APM_URL"))
	}
	if tagsDiffer(cfg.HostMetadata.Tags, oldCfg.HostMetadata.Tags) {
		warnings = append(warnings, fmt.Errorf("\"tags\" does not default to \"DD_TAGS\"'s value since v0.50.0. Use 'env' configuration source instead"))

	}

	if len(warnings) > 0 {
		warnings = append(warnings, errIssue)
	}
	return
}
