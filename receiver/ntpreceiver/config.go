// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ntpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver"

import (
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver/internal/metadata"
)

// Config is the configuration for the NSX receiver
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Endpoint                       string `mapstructure:"endpoint"`
}
