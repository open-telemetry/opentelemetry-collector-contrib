// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipvsreceiver // import "github.com/sergeysedoy97/opentelemetry-collector-contrib/receiver/ipvsreceiver"

import (
	"github.com/sergeysedoy97/opentelemetry-collector-contrib/receiver/ipvsreceiver/internal/metadata"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
}
