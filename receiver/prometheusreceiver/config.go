// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	commonconfig "github.com/prometheus/common/config"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/targetallocator"
)

// Config defines configuration for Prometheus receiver.
type Config struct {
	PrometheusConfig   *PromConfig `mapstructure:"config"`
	TrimMetricSuffixes bool        `mapstructure:"trim_metric_suffixes"`

	// ReportExtraScrapeMetrics - enables reporting of additional metrics for Prometheus client like scrape_body_size_bytes
	//
	// Deprecated: use the feature gate "receiver.prometheusreceiver.EnableReportExtraScrapeMetrics" instead.
	ReportExtraScrapeMetrics bool `mapstructure:"report_extra_scrape_metrics"`

	TargetAllocator configoptional.Optional[targetallocator.Config] `mapstructure:"target_allocator"`

	//  APIServer has the settings to enable the receiver to host the Prometheus API
	// server in agent mode. This allows the user to call the endpoint to get
	// the config, service discovery, and targets for debugging purposes.
	APIServer APIServer `mapstructure:"api_server"`

	// For testing only.
	ignoreMetadata bool
}

// Validate checks the receiver configuration is valid.
func (cfg *Config) Validate() error {
	if !cfg.PrometheusConfig.ContainsScrapeConfigs() && !cfg.TargetAllocator.HasValue() {
		return errors.New("no Prometheus scrape_configs or target_allocator set")
	}

	if err := cfg.APIServer.Validate(); err != nil {
		return fmt.Errorf("invalid API server configuration settings: %w", err)
	}

	return nil
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

// copyStaticConfig copies static service discovery configs from src to dst to assure that labels in StaticConfig are retained.
func copyStaticConfig(dst *PromConfig, src any) {
	// only deal with PromConfig
	srcCfg, ok := src.(*PromConfig)
	if !ok {
		return
	}

	// Job name -> static config list
	// The static configs are grouped by job name, so that we can
	// copy them over to the destination config.
	srcCfgMap := make(map[string][]*targetgroup.Group, len(srcCfg.ScrapeConfigs))
	for _, srcSC := range srcCfg.ScrapeConfigs {
		for _, srcSDC := range srcSC.ServiceDiscoveryConfigs {
			if sc, ok := srcSDC.(discovery.StaticConfig); ok {
				srcCfgMap[srcSC.JobName] = append(srcCfgMap[srcSC.JobName], sc...)
			}
		}
	}

	for _, dstSC := range dst.ScrapeConfigs {
		if len(srcCfgMap[dstSC.JobName]) == 0 {
			continue
		}

		dstSC.ServiceDiscoveryConfigs = slices.DeleteFunc(dstSC.ServiceDiscoveryConfigs, func(cfg discovery.Config) bool {
			// Remove all static configs for this job name.
			_, ok := cfg.(discovery.StaticConfig)
			return ok
		})
		dstSC.ServiceDiscoveryConfigs = append(dstSC.ServiceDiscoveryConfigs, discovery.StaticConfig(srcCfgMap[dstSC.JobName]))
	}
}

func reloadPromConfig(dst *PromConfig, src any) error {
	yamlOut, err := yaml.MarshalWithOptions(
		src,
		yaml.CustomMarshaler(func(s commonconfig.Secret) ([]byte, error) {
			return yaml.Marshal(string(s))
		}),
	)
	if err != nil {
		return fmt.Errorf("prometheus receiver: failed to marshal config to yaml: %w", err)
	}

	newCfg, err := promconfig.Load(string(yamlOut), slog.Default())
	if err != nil {
		return fmt.Errorf("prometheus receiver: failed to unmarshal yaml to prometheus config object: %w", err)
	}
	copyStaticConfig((*PromConfig)(newCfg), src)
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
