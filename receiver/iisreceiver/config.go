// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

// Config defines configuration for simple prometheus receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	MetricsBuilderConfig           metadata.MetricsBuilderConfig `mapstructure:",squash"`
}
