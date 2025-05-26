// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zookeeperreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver"

import (
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/scraper/zookeeperscraper"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	zookeeperscraper.Config        `mapstructure:",squash"`
}
