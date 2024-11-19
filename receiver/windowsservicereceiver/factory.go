// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func createDefaultConfig() component.Config {
	scfg := scraperhelper.NewDefaultControllerConfig()

	return &Config{
		ControllerConfig: scfg,
	}
}
