// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <hostname>:<port>`)

// CertificateTarget represents a target for certificate checking, which can be either
// a network endpoint or a local file
type CertificateTarget struct {
	confignet.TCPAddrConfig `mapstructure:",squash"`
	FilePath                string `mapstructure:"file_path"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets                        []*CertificateTarget `mapstructure:"targets"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func validateTarget(ct *CertificateTarget) error {
	if ct.Endpoint != "" && ct.FilePath != "" {
		return errors.New("cannot specify both endpoint and file_path")
	}
	if ct.Endpoint == "" && ct.FilePath == "" {
		return errors.New("must specify either endpoint or file_path")
	}
	return nil
}

func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Targets) == 0 {
		err = multierr.Append(err, errMissingTargets)
	}

	for _, target := range cfg.Targets {
		err = multierr.Append(err, validateTarget(target))
	}

	return err
}

func (cfg *Config) ValidateForDiscovery(rawCfg map[string]any, discoveredEndpoint string) error {
	targets, ok := rawCfg["targets"].([]any)
	if !ok {
		return nil
	}

	for i, t := range targets {
		target, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if endpoint, exists := target["endpoint"]; exists {
			endpointStr, ok := endpoint.(string)
			if !ok {
				return fmt.Errorf("targets[%d].endpoint: expected string", i)
			}

			if endpointStr != discoveredEndpoint {
				return fmt.Errorf("targets[%d].endpoint %q does not match discoveredEndpoint %q", i, endpointStr, discoveredEndpoint)
			}
		}
	}
	return nil
}
