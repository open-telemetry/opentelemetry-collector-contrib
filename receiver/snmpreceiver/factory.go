// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	typeStr   = "snmp"
	stability = component.StabilityLevelAlpha
)

var errConfigNotSNMP = errors.New("config was not a SNMP receiver config")

// NewFactory creates a new receiver factory for SNMP
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

// createDefaultConfig creates a config for SNMP with as many default values as possible
func createDefaultConfig() component.Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: defaultCollectionInterval,
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
	params receiver.CreateSettings,
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
	scraper, err := scraperhelper.NewScraper(typeStr, snmpScraper.scrape, scraperhelper.WithStart(snmpScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&snmpConfig.ScraperControllerSettings, params, consumer, scraperhelper.AddScraper(scraper))
}

// addMissingConfigDefaults adds any missing comfig parameters that have defaults
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

	return component.ValidateConfig(cfg)
}
