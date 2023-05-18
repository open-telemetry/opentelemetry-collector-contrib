// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

type Config struct {
	DataSource                              string `mapstructure:"datasource"`
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
}

func (c Config) Validate() error {
	if c.DataSource == "" {
		return errors.New("'datasource' cannot be empty")
	}
	if _, err := url.Parse(c.DataSource); err != nil {
		return fmt.Errorf("'datasource' is invalid: %w", err)
	}
	return nil
}
