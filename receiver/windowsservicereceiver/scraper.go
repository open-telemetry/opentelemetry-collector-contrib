// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
)

type windowsServiceScraper struct {
	scm      serviceManager
	settings receiver.Settings
	conf     *Config
	mb       *metadata.MetricsBuilder
}

func newWindowsServiceScraper(settings receiver.Settings, cfg *Config) windowsServiceScraper {
	return windowsServiceScraper{
		settings: settings,
		mb:       metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
	}
}

func (ws *windowsServiceScraper) start(ctx context.Context, _ component.Host) (err error) {
	return nil
}

func (ws *windowsServiceScraper) shutdown(ctx context.Context) (err error) {
	return nil
}

func (ws *windowsServiceScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	return ws.mb.Emit(), nil
}
