// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

var errConfigNotSNMP = errors.New("config was not a SNMP receiver config")

// NewFactory creates a new receiver factory for SNMP
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

// createDefaultConfig creates a config for SNMP with as many default values as possible
func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: defaultCollectionInterval,
			Timeout:            defaultTimeout,
		},
		Endpoint:      defaultEndpoint,
		Version:       defaultVersion,
		Community:     defaultCommunity,
		SecurityLevel: defaultSecurityLevel,
		AuthType:      defaultAuthType,
		PrivacyType:   defaultPrivacyType,
	}
}

// createMetricsReceiver creates the metric receiver for SNMP
func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	snmpConfig, ok := config.(*Config)
	if !ok {
		return nil, errConfigNotSNMP
	}

	if err := addMissingConfigDefaults(snmpConfig); err != nil {
		return nil, fmt.Errorf("failed to validate added config defaults: %w", err)
	}

	snmpScraper := newScraper(params.Logger, snmpConfig, params)
	s, err := scraper.NewMetrics(snmpScraper.scrape, scraper.WithStart(snmpScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(&snmpConfig.ControllerConfig, params, consumer, scraperhelper.AddScraper(metadata.Type, s))
}

// addMissingConfigDefaults adds any missing config parameters that have defaults
func addMissingConfigDefaults(cfg *Config) error {
	// Add the schema prefix to the endpoint if it doesn't contain one
	if !strings.Contains(cfg.Endpoint, "://") {
		cfg.Endpoint = "udp://" + cfg.Endpoint
	}

	// Add default port to endpoint if it doesn't contain one
	u, err := url.Parse(cfg.Endpoint)
	if err == nil && u.Port() == "" {
		portSuffix := "161"
		if cfg.Endpoint[len(cfg.Endpoint)-1:] != ":" {
			portSuffix = ":" + portSuffix
		}
		cfg.Endpoint += portSuffix
	}

	// Set defaults for metric configs
	for _, metricCfg := range cfg.Metrics {
		if metricCfg.Unit == "" {
			metricCfg.Unit = "1"
		}
		if metricCfg.Gauge != nil && metricCfg.Gauge.ValueType == "" {
			metricCfg.Gauge.ValueType = "double"
		}
		if metricCfg.Sum != nil {
			if metricCfg.Sum.ValueType == "" {
				metricCfg.Sum.ValueType = "double"
			}
			if metricCfg.Sum.Aggregation == "" {
				metricCfg.Sum.Aggregation = "cumulative"
			}
		}
	}

	return xconfmap.Validate(cfg)
}
