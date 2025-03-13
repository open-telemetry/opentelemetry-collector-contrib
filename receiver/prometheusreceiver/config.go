// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	commonconfig "github.com/prometheus/common/config"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"
)

// Config defines configuration for Prometheus receiver.
type Config struct {
	PrometheusConfig   *PromConfig `mapstructure:"config"`
	TrimMetricSuffixes bool        `mapstructure:"trim_metric_suffixes"`
	// UseStartTimeMetric enables retrieving the start time of all counter metrics
	// from the process_start_time_seconds metric. This is only correct if all counters on that endpoint
	// started after the process start time, and the process is the only actor exporting the metric after
	// the process started. It should not be used in "exporters" which export counters that may have
	// started before the process itself. Use only if you know what you are doing, as this may result
	// in incorrect rate calculations.
	UseStartTimeMetric   bool   `mapstructure:"use_start_time_metric"`
	StartTimeMetricRegex string `mapstructure:"start_time_metric_regex"`

	// ReportExtraScrapeMetrics - enables reporting of additional metrics for Prometheus client like scrape_body_size_bytes
	ReportExtraScrapeMetrics bool `mapstructure:"report_extra_scrape_metrics"`

	TargetAllocator *targetallocator.Config `mapstructure:"target_allocator"`

	//  APIServer has the settings to enable the receiver to host the Prometheus API
	// server in agent mode. This allows the user to call the endpoint to get
	// the config, service discovery, and targets for debugging purposes.
	APIServer *APIServer `mapstructure:"api_server"`
}

// Validate checks the receiver configuration is valid.
func (cfg *Config) Validate() error {
	if !containsScrapeConfig(cfg) && cfg.TargetAllocator == nil {
		return errors.New("no Prometheus scrape_configs or target_allocator set")
	}

	if cfg.APIServer != nil {
		if err := cfg.APIServer.Validate(); err != nil {
			return fmt.Errorf("invalid API server configuration settings: %w", err)
		}
	}

	return nil
}

func containsScrapeConfig(cfg *Config) bool {
	if cfg.PrometheusConfig == nil {
		return false
	}
	scrapeConfigs, err := (*promconfig.Config)(cfg.PrometheusConfig).GetScrapeConfigs()
	if err != nil {
		return false
	}

	return len(scrapeConfigs) > 0
}

// PromConfig is a redeclaration of promconfig.Config because we need custom unmarshaling
// as prometheus "config" uses `yaml` tags.
type PromConfig promconfig.Config

var _ confmap.Unmarshaler = (*PromConfig)(nil)

func (cfg *PromConfig) Unmarshal(componentParser *confmap.Conf) error {
	cfgMap := componentParser.ToStringMap()
	if len(cfgMap) == 0 {
		return nil
	}
	return unmarshalYAML(cfgMap, (*promconfig.Config)(cfg))
}

func (cfg *PromConfig) Validate() error {
	// Reject features that Prometheus supports but that the receiver doesn't support:
	// See:
	// * https://github.com/open-telemetry/opentelemetry-collector/issues/3863
	// * https://github.com/open-telemetry/wg-prometheus/issues/3
	unsupportedFeatures := make([]string, 0, 4)
	if len(cfg.RemoteWriteConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "remote_write")
	}
	if len(cfg.RemoteReadConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "remote_read")
	}
	if len(cfg.RuleFiles) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "rule_files")
	}
	if len(cfg.AlertingConfig.AlertRelabelConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "alert_config.relabel_configs")
	}
	if len(cfg.AlertingConfig.AlertmanagerConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "alert_config.alertmanagers")
	}
	if len(unsupportedFeatures) != 0 {
		// Sort the values for deterministic error messages.
		sort.Strings(unsupportedFeatures)
		return fmt.Errorf("unsupported features:\n\t%s", strings.Join(unsupportedFeatures, "\n\t"))
	}

	scrapeConfigs, err := (*promconfig.Config)(cfg).GetScrapeConfigs()
	if err != nil {
		return err
	}
	// Since Prometheus 3.0, the scrape manager started to fail scrapes that don't have proper
	// Content-Type headers, but they provided an extra configuration option to fallback to the
	// previous behavior. We need to make sure that this option is set for all scrape configs
	// to avoid introducing a breaking change.
	for _, sc := range scrapeConfigs {
		if sc.ScrapeFallbackProtocol == "" {
			sc.ScrapeFallbackProtocol = promconfig.PrometheusText0_0_4
		}
	}

	for _, sc := range scrapeConfigs {
		if err := validateHTTPClientConfig(&sc.HTTPClientConfig); err != nil {
			return err
		}

		for _, c := range sc.ServiceDiscoveryConfigs {
			if c, ok := c.(*kubernetes.SDConfig); ok {
				if err := validateHTTPClientConfig(&c.HTTPClientConfig); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func unmarshalYAML(in map[string]any, out any) error {
	yamlOut, err := yaml.Marshal(in)
	if err != nil {
		return fmt.Errorf("prometheus receiver: failed to marshal config to yaml: %w", err)
	}

	err = yaml.UnmarshalStrict(yamlOut, out)
	if err != nil {
		return fmt.Errorf("prometheus receiver: failed to unmarshal yaml to prometheus config object: %w", err)
	}
	return nil
}

func validateHTTPClientConfig(cfg *commonconfig.HTTPClientConfig) error {
	if cfg.Authorization != nil {
		if err := checkFile(cfg.Authorization.CredentialsFile); err != nil {
			return fmt.Errorf("error checking authorization credentials file %q: %w", cfg.Authorization.CredentialsFile, err)
		}
	}

	if err := checkTLSConfig(cfg.TLSConfig); err != nil {
		return err
	}
	return nil
}

func checkFile(fn string) error {
	// Nothing set, nothing to error on.
	if fn == "" {
		return nil
	}
	_, err := os.Stat(fn)
	return err
}

func checkTLSConfig(tlsConfig commonconfig.TLSConfig) error {
	if err := checkFile(tlsConfig.CertFile); err != nil {
		return fmt.Errorf("error checking client cert file %q: %w", tlsConfig.CertFile, err)
	}
	if err := checkFile(tlsConfig.KeyFile); err != nil {
		return fmt.Errorf("error checking client key file %q: %w", tlsConfig.KeyFile, err)
	}
	return nil
}

type APIServer struct {
	Enabled      bool                    `mapstructure:"enabled"`
	ServerConfig confighttp.ServerConfig `mapstructure:"server_config"`
}

func (cfg *APIServer) Validate() error {
	if !cfg.Enabled {
		return nil
	}

	if cfg.ServerConfig.Endpoint == "" {
		return fmt.Errorf("if api_server is enabled, it requires a non-empty server_config endpoint")
	}

	return nil
}
