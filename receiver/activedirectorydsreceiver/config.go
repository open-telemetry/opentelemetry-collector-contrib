// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectorydsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver"

import (
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver/internal/metadata"
)

type Config struct {
	ControllerConfig     scraperhelper.ControllerConfig `mapstructure:",squash"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig  `mapstructure:",squash"`
}
