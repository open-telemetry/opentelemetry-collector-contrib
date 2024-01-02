// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package haproxyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver/internal/metadata"
)

type Config struct {
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
}

func (c Config) Validate() error {
	if c.HTTPClientSettings.Endpoint == "" {
		return errors.New("'endpoint' cannot be empty")
	}
	return nil
}
