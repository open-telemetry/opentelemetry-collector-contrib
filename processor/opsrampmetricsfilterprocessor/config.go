// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsfilterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampmetricsfilterprocessor"

import (
	"os"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the Alert Metrics Extractor processor.
type Config struct {
	// AlertConfigMapName is the name of the ConfigMap containing alert definitions
	// Optional: defaults to "opsramp-alert-user-config"
	AlertConfigMapName string `mapstructure:"alert_definitions_configmap_name"`

	// AlertConfigMapKey is the key in the ConfigMap containing alert definitions YAML
	// Optional: defaults to "alert-definitions.yaml"
	AlertConfigMapKey string `mapstructure:"alert_definitions_key"`

	// Namespace is the Kubernetes namespace where ConfigMaps are located
	// This is automatically populated from the NAMESPACE environment variable
	// Users should not set this field directly
	Namespace string `mapstructure:"namespace"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	// Set defaults if not provided
	if cfg.AlertConfigMapName == "" {
		cfg.AlertConfigMapName = "opsramp-alert-user-config"
	}

	if cfg.AlertConfigMapKey == "" {
		cfg.AlertConfigMapKey = "alert-definitions.yaml"
	}

	// Set namespace from environment variable if not already set
	cfg.Namespace = os.Getenv("NAMESPACE")
	if cfg.Namespace == "" {
		cfg.Namespace = "opsramp-agent"
	}

	return nil
}
