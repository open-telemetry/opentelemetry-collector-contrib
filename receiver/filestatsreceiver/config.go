// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	Include                                 string `mapstructure:"include"`
}

func (c Config) Validate() error {
	if c.Include == "" {
		return errors.New("include must not be empty")
	}
	return nil
}
