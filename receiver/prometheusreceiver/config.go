// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	commonconfig "github.com/prometheus/common/config"
	model "github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"

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

	// Endpoint field is included for unmarshaling compatibility with receiver_creator.
	// Discoverable receivers do not use this field as they handle endpoint validation internally.
	// This field is ignored during processing and should remain empty for discoverable receivers.
	Endpoint string `mapstructure:"endpoint,omitempty"`
}

// Validate checks the receiver configuration is valid.
func (cfg *Config) Validate() error {
	if !cfg.PrometheusConfig.ContainsScrapeConfigs() && cfg.TargetAllocator == nil {
		return errors.New("no Prometheus scrape_configs or target_allocator set")
	}

	if cfg.APIServer != nil {
		if err := cfg.APIServer.Validate(); err != nil {
			return fmt.Errorf("invalid API server configuration settings: %w", err)
		}
	}

	return nil
}

// ValidateDiscovery validates the rawCfg provided via discovery annotations and
// ensures it would only scrape the discovered endpoint.
// This validation occurs AFTER template substitution, so all template expressions
// like `endpoint` have already been expanded to concrete values.
func (cfg *Config) ValidateDiscovery(rawCfg map[string]any, discoveredEndpoint string) error {
	// Create a temporary config to unmarshal the configuration
	tempCfg := &Config{}
	if err := tempCfg.unmarshalFromMap(rawCfg); err != nil {
		return fmt.Errorf("failed to unmarshal prometheus config: %w", err)
	}

	// Ensure the endpoint field is not set for discoverable receivers
	if tempCfg.Endpoint != "" {
		return errors.New("endpoint field should not be set for discoverable receivers - targets are validated directly in prometheus config")
	}

	// Validate that the prometheus config only targets the discovered endpoint
	return validatePrometheusTargets(tempCfg.PrometheusConfig, discoveredEndpoint)
}

// unmarshalFromMap unmarshals a map into the Config struct
func (cfg *Config) unmarshalFromMap(rawCfg map[string]any) error {
	// Handle the endpoint field if present
	if endpoint, exists := rawCfg["endpoint"]; exists {
		if endpointStr, ok := endpoint.(string); ok {
			cfg.Endpoint = endpointStr
		}
	}

	// Handle the nested "config" field which contains the actual prometheus configuration
	if configMap, exists := rawCfg["config"]; exists {
		promCfgMap, ok := configMap.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid config field format: expected map[string]any but got %T", configMap)
		}
		cfg.PrometheusConfig = &PromConfig{}
		return reloadPromConfig(cfg.PrometheusConfig, promCfgMap)
	}
	return errors.New("missing prometheus config field")
}

// validatePrometheusTargets validates that all targets in the prometheus configuration
// only target the discovered endpoint, preventing scraping from unintended sources.
func validatePrometheusTargets(promCfg *PromConfig, discoveredEndpoint string) error {
	if promCfg == nil {
		return errors.New("prometheus configuration is nil")
	}

	// In discovery mode, disallow externalized configs to prevent bypassing validation
	if len(promCfg.ScrapeConfigFiles) > 0 {
		return errors.New("scrape_config_files are not allowed in discovery mode")
	}

	// Validate all scrape configs
	for i, scrapeConfig := range promCfg.ScrapeConfigs {
		if err := validateScrapeConfig(scrapeConfig, discoveredEndpoint, i); err != nil {
			return err
		}
	}

	return nil
}

// validateScrapeConfig validates a single scrape configuration
func validateScrapeConfig(scrapeConfig *promconfig.ScrapeConfig, discoveredEndpoint string, index int) error {
	if scrapeConfig == nil {
		return fmt.Errorf("scrape config %d is nil", index)
	}

	// Allow only static discovery; reject all other discovery mechanisms in discovery mode
	for sdIdx, sd := range scrapeConfig.ServiceDiscoveryConfigs {
		switch cfg := sd.(type) {
		case discovery.StaticConfig:
			// Validate each target group and target address
			for tgIdx, tg := range cfg {
				for tIdx, lbls := range tg.Targets {
					addr := string(lbls[model.AddressLabel])
					if err := validatePrometheusTarget(addr, discoveredEndpoint); err != nil {
						return fmt.Errorf("invalid target %d in target_group %d of sd[%d] in scrape_config %d: %w", tIdx, tgIdx, sdIdx, index, err)
					}
				}
			}
		default:
			return fmt.Errorf("service discovery type %T is not allowed in discovery mode (scrape_config index %d)", sd, index)
		}
	}

	return nil
}

// validatePrometheusTarget validates that a prometheus target matches the discovered endpoint.
// At this point, all template expressions have been expanded to concrete values by expandConfig().
func validatePrometheusTarget(target, discoveredEndpoint string) error {
	if target == "" {
		return errors.New("target cannot be empty")
	}

	// Extract host information from the target
	targetHost := extractHostFromTarget(target)
	if targetHost == "" {
		return errors.New("could not extract host from target")
	}

	// The target host must exactly match the discovered endpoint
	if targetHost != discoveredEndpoint {
		return fmt.Errorf("target host %s does not match discovered endpoint %s", targetHost, discoveredEndpoint)
	}

	return nil
}

// extractHostFromTarget extracts the host portion from a prometheus target
func extractHostFromTarget(target string) string {
	// Handle various target formats:
	// - "host:port" (most common)
	// - "http://host:port/path" (URL format)
	// - "https://host:port/path" (HTTPS URL format)

	// First, try to parse as URL
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		if u, err := url.Parse(target); err == nil && u.Host != "" {
			return u.Host
		}
	}

	// If not a full URL, treat as host:port and return as-is since prometheus targets are typically host:port
	// This handles both IPv4 (host:port) and IPv6 ([::1]:port) formats
	return target
}

// PromConfig is a redeclaration of promconfig.Config because we need custom unmarshaling
// as prometheus "config" uses `yaml` tags.
type PromConfig promconfig.Config

var _ confmap.Unmarshaler = (*PromConfig)(nil)

// ContainsScrapeConfigs returns true if the Prometheus config contains any scrape configs.
func (cfg *PromConfig) ContainsScrapeConfigs() bool {
	return cfg != nil && (len(cfg.ScrapeConfigs) > 0 || len(cfg.ScrapeConfigFiles) > 0)
}

func (cfg *PromConfig) Reload() error {
	return reloadPromConfig(cfg, cfg)
}

func (cfg *PromConfig) Unmarshal(componentParser *confmap.Conf) error {
	cfgMap := componentParser.ToStringMap()
	if len(cfgMap) == 0 {
		return nil
	}

	// Handle discovery annotations: if scrape_configs is at root level,
	// this is likely from a discovery annotation and should be passed directly
	if _, hasPrometheusConfig := cfgMap["scrape_configs"]; hasPrometheusConfig {
		return reloadPromConfig(cfg, cfgMap)
	}

	// Handle nested structure from discovery annotations
	if _, hasGlobalConfig := cfgMap["global"]; hasGlobalConfig {
		return reloadPromConfig(cfg, cfgMap)
	}

	// For other nested structures, try to extract prometheus config
	// This handles cases where config might be nested under another key
	for k, v := range cfgMap {
		if k != "scrape_configs" && k != "global" {
			if promCfgMap, ok := v.(map[string]any); ok {
				if _, hasPrometheusFields := promCfgMap["scrape_configs"]; hasPrometheusFields {
					return reloadPromConfig(cfg, promCfgMap)
				}
			}
		}
	}

	return reloadPromConfig(cfg, cfgMap)
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

	if cfg.ContainsScrapeConfigs() {
		scrapeConfigs, err := (*promconfig.Config)(cfg).GetScrapeConfigs()
		if err != nil {
			return fmt.Errorf("failed to get scrape configs: %w", err)
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
	}

	return nil
}

func reloadPromConfig(dst *PromConfig, src any) error {
	yamlOut, err := yaml.MarshalWithOptions(
		src,
		yaml.CustomMarshaler(func(s commonconfig.Secret) ([]byte, error) {
			return []byte(s), nil
		}),
	)
	if err != nil {
		return fmt.Errorf("prometheus receiver: failed to marshal config to yaml: %w", err)
	}

	newCfg, err := promconfig.Load(string(yamlOut), slog.Default())
	if err != nil {
		return fmt.Errorf("prometheus receiver: failed to unmarshal yaml to prometheus config object: %w", err)
	}
	*dst = PromConfig(*newCfg)
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
		return errors.New("if api_server is enabled, it requires a non-empty server_config endpoint")
	}

	return nil
}
