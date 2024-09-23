// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	defaultInterval = 5 * time.Minute
)

func createDefaultConfig() component.Config {
	scfg := scraperhelper.NewDefaultControllerConfig()
	scfg.CollectionInterval = defaultInterval

	return &Config{
		ControllerConfig: scfg,
		Services:         nil,
		MonitorAll:       false,
	}
}
