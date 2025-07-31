// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//revive:disable:unused-parameter
//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
)

//nolint:unused
type windowsServiceScraper struct {
	scm      serviceManager
	settings receiver.Settings
	conf     *Config
	mb       *metadata.MetricsBuilder
}

//nolint:unused
func newWindowsServiceScraper(settings receiver.Settings, _ *Config) windowsServiceScraper {
	return windowsServiceScraper{
		settings: settings,
		mb:       metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
	}
}

//nolint:unused
func (*windowsServiceScraper) start(context.Context, component.Host) (err error) {
	return nil
}

//nolint:unused
func (*windowsServiceScraper) shutdown(context.Context) (err error) {
	return nil
}

//nolint:unused
func (ws *windowsServiceScraper) scrape(context.Context) (pmetric.Metrics, error) {
	return ws.mb.Emit(), nil
}
