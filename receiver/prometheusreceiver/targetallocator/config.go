// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package targetallocator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	commonconfig "github.com/prometheus/common/config"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v2"
)

type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	Interval                time.Duration         `mapstructure:"interval"`
	CollectorID             string                `mapstructure:"collector_id"`
	HTTPSDConfig            *PromHTTPSDConfig     `mapstructure:"http_sd_config"`
	HTTPScrapeConfig        *PromHTTPClientConfig `mapstructure:"http_scrape_config"`
}

// PromHTTPSDConfig is a redeclaration of promHTTP.SDConfig because we need custom unmarshaling
// as prometheus "config" uses `yaml` tags.
type PromHTTPSDConfig promHTTP.SDConfig

func (cfg *Config) Validate() error {
	// ensure valid endpoint
	if _, err := url.ParseRequestURI(cfg.Endpoint); err != nil {
		return fmt.Errorf("TargetAllocator endpoint is not valid: %s", cfg.Endpoint)
	}
	// ensure valid collectorID without variables
	if cfg.CollectorID == "" || strings.Contains(cfg.CollectorID, "${") {
		return fmt.Errorf("CollectorID is not a valid ID")
	}

	return nil
}

var _ confmap.Unmarshaler = (*PromHTTPSDConfig)(nil)

func (cfg *PromHTTPSDConfig) Unmarshal(componentParser *confmap.Conf) error {
	cfgMap := componentParser.ToStringMap()
	if len(cfgMap) == 0 {
		return nil
	}
	cfgMap["url"] = "http://placeholder" // we have to set it as else marshaling will fail
	return unmarshalYAML(cfgMap, (*promHTTP.SDConfig)(cfg))
}

type PromHTTPClientConfig commonconfig.HTTPClientConfig

var _ confmap.Unmarshaler = (*PromHTTPClientConfig)(nil)

func (cfg *PromHTTPClientConfig) Unmarshal(componentParser *confmap.Conf) error {
	cfgMap := componentParser.ToStringMap()
	if len(cfgMap) == 0 {
		return nil
	}
	return unmarshalYAML(cfgMap, (*commonconfig.HTTPClientConfig)(cfg))
}

func (cfg *PromHTTPClientConfig) Validate() error {
	httpCfg := (*commonconfig.HTTPClientConfig)(cfg)
	if err := validateHTTPClientConfig(httpCfg); err != nil {
		return err
	}
	// Prometheus UnmarshalYaml implementation by default calls Validate,
	// but it is safer to do it here as well.
	return httpCfg.Validate()
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
