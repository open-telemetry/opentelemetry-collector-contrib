// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"

import (
	"fmt"
	"path/filepath"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver/internal/metadata"
)

// Config defines the configuration for the AWS EFA receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`

	// HostPath is the root path to the host filesystem when running in a container.
	// Defaults to "" (direct host access). Set to "/host" or "/rootfs" when
	// the host filesystem is mounted into the container.
	HostPath string `mapstructure:"host_path"`
}

var _ component.Config = (*Config)(nil)

// Validate checks that the receiver configuration is valid. It returns an error
// if HostPath is set but is not an absolute path.
func (c *Config) Validate() error {
	if c.HostPath != "" && !filepath.IsAbs(c.HostPath) {
		return fmt.Errorf("host_path must be an absolute path, got %q", c.HostPath)
	}
	return nil
}
