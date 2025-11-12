// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

import "errors"

// OrchestratorExplorerConfig defines configuration for the Datadog orchestrator explorer.
type OrchestratorExplorerConfig struct {
	// Enabled enables the orchestrator explorer.
	Enabled bool `mapstructure:"enabled"`

	// ClusterName defines the kubernetes cluster name.
	ClusterName string `mapstructure:"cluster_name"`
}

// Validate the configuration for errors.
func (c *OrchestratorExplorerConfig) Validate() error {
	if c.Enabled && c.ClusterName == "" {
		return errors.New("'cluster_name' is required when 'orchestrator_explorer' is enabled")
	}
	return nil
}
