// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

// Config defines configuration for a sqlserver receiver.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	InstanceName                            string `mapstructure:"instance_name"`
	ComputerName                            string `mapstructure:"computer_name"`
}

func (cfg *Config) Validate() error {
	if cfg.InstanceName != "" && cfg.ComputerName == "" {
		return fmt.Errorf("'instance_name' may not be specified without 'computer_name'")
	}
	if cfg.InstanceName == "" && cfg.ComputerName != "" {
		return fmt.Errorf("'computer_name' may not be specified without 'instance_name'")
	}

	return nil
}
