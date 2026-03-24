// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsekshyperpodreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsekshyperpodreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

// Config defines the configuration for the HyperPod health receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// ClusterName is the name of the Kubernetes cluster.
	// If empty, the cluster_name attribute is omitted from metrics.
	ClusterName string `mapstructure:"cluster_name"`
}

var _ component.Config = (*Config)(nil)

// Validate checks that the configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.CollectionInterval <= 0 {
		return errors.New("collection_interval must be positive")
	}
	return nil
}
