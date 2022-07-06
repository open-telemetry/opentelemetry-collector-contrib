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

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// futureDefaultConfig returns the future default config we will use from v0.50 onwards.
func futureDefaultConfig() *Config {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("datadog")),
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: 15 * time.Second,
		},
		API: APIConfig{
			Site: "datadoghq.com",
		},
		TagsConfig: TagsConfig{
			Env: "none",
		},
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.com",
			},
			SendMonotonic: true,
			DeltaTTL:      3600,
			Quantiles:     true,
			ExporterConfig: MetricsExporterConfig{
				ResourceAttributesAsTags:             false,
				InstrumentationLibraryMetadataAsTags: false,
				InstrumentationScopeMetadataAsTags:   false,
			},
			HistConfig: HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode: CumulativeMonotonicSumModeToDelta,
			},
			SummaryConfig: SummaryConfig{
				Mode: SummaryModeGauges,
			},
		},
		Traces: TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			IgnoreResources: []string{},
		},
		HostMetadata: HostMetadataConfig{
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
		"%q will not default to %q's value in a future minor version. Set %s: ${%s} to remove this warning",
		settingName,
		envVarName,
		settingName,
		envVarName,
	)
}

// tagsDiffer checks if the host tags from each configuration are different.
func tagsDiffer(cfgTags []string, futureTags []string) bool {
	if len(cfgTags) != len(futureTags) {
		return true
	}

	oldCfgSet := map[string]struct{}{}
	for _, tag := range cfgTags {
		oldCfgSet[tag] = struct{}{}
	}

	for _, tag := range futureTags {
		if _, ok := oldCfgSet[tag]; !ok {
			// tag is in old but not new
			return true
		}
	}

	return false
}

// warnUseOfEnvVars returns warnings related to automatic environment variable detection.
// Right now, we automatically get the value for e.g. the API key from DD_API_KEY, even if
// the user is not doing `api.key: ${DD_API_KEY}`. We are going to remove this functionality.
func warnUseOfEnvVars(configMap *confmap.Conf, cfg *Config) (warnings []error) {
	// We don't see the raw YAML contents from our exporter so, to compare with the new default,
	// we unmarshal the config map on the future default config and compare the two structs.
	// Any differences will be due to the change in default configuration.
	futureCfg := futureDefaultConfig()
	err := configMap.UnmarshalExact(futureCfg)

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
	if cfg.API.Key != futureCfg.API.Key {
		warnings = append(warnings, errUsedEnvVar("api.key", "DD_API_KEY"))
	}
	if cfg.API.Site != futureCfg.API.Site {
		warnings = append(warnings, errUsedEnvVar("api.site", "DD_SITE"))
	}
	if cfg.Hostname != futureCfg.Hostname {
		warnings = append(warnings, errUsedEnvVar("hostname", "DD_HOST"))
	}
	if cfg.Env != futureCfg.Env {
		warnings = append(warnings, errUsedEnvVar("env", "DD_ENV"))
	}
	if cfg.Service != futureCfg.Service {
		warnings = append(warnings, errUsedEnvVar("service", "DD_SERVICE"))
	}
	if cfg.Version != futureCfg.Version {
		warnings = append(warnings, errUsedEnvVar("version", "DD_VERSION"))
	}
	if cfg.Metrics.Endpoint != futureCfg.Metrics.Endpoint {
		warnings = append(warnings, errUsedEnvVar("metrics.endpoint", "DD_URL"))
	}
	if cfg.Traces.Endpoint != futureCfg.Traces.Endpoint {
		warnings = append(warnings, errUsedEnvVar("traces.endpoint", "DD_APM_URL"))
	}
	if tagsDiffer(cfg.getHostTags(), futureCfg.getHostTags()) {
		warnings = append(warnings, fmt.Errorf("\"tags\" will not default to \"DD_TAGS\"'s value in a future minor version. Use 'env' configuration source instead to remove this warning"))
	}

	if len(warnings) > 0 {
		warnings = append(warnings, errIssue)
	}
	return
}
