// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nginxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver"

import (
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	confighttp.ClientConfig        `mapstructure:",squash"`
	MetricsBuilderConfig           metadata.MetricsBuilderConfig `mapstructure:",squash"`

	// prevent unkeyed literal initialization
	_ struct{}
}
