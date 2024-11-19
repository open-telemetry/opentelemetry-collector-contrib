// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

type windowsServiceScraper struct {
	cfg      *Config
	settings receiver.Settings
	mgr      *Manager
	services []string
	mb       *metadata.MetricsBuilder
}

func newWindowsServiceScraper(settings receiver.Settings, cfg *Config) windowsServiceScraper {
	return windowsServiceScraper{
		cfg:      cfg,
		settings: settings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

// start is run once on service start up and initiates the connection to the Service Manager
// by getting a Handler. It also creates (retrieves and then filters) the list of services to be monitored by
// the receiver.
func (wss *windowsServiceScraper) start(ctx context.Context, _ component.Host) error {
	h, err := SCConnect()
	if err != nil {
		return err
	}

	wss.mgr = h

	s := wss.cfg.IncludeServices
	if s == nil {
		s, err = wss.mgr.ListServices()
		if err != nil {
			return err
		}

		if wss.cfg.ExcludeServices != nil {
			s = filterServices(s, wss.cfg)
		}
	}

	wss.services = s
	return nil
}

// if the user has configured ExcludeServices list filter these services out. Might naively
// look as follows. Since this is only called once at start I think this performance should be fine.
func filterServices(slist []string, cfg *Config) []string {
	var res []string
	var f bool

	for _, v := range slist {
		f = false
		for _, fv := range cfg.ExcludeServices {
			if v == fv {
				f = true
				break
			}
		}
		if !f {
			res = append(res, v)
		}
	}

	return res
}

// TODO: scraperhelper shutdown function run on service termination, closes the connection
// to the windows service manager
func (wss *windowsServiceScraper) shutdown(_ context.Context) error {

	err := wss.mgr.Disconnect()
	return err
}

func (wss *windowsServiceScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	// TODO: implement the actual scrape function. Registered in the factory and controlled by scrapercontroller
	var errs *scrapererror.ScrapeErrors
	now := pcommon.NewTimestampFromTime(time.Now())

	for _, s := range wss.services {
		m, err := GetService(wss.mgr, s)
		if err != nil {
			errs.Add(err)
			continue
		}
		wss.mb.RecordWindowsServiceDataPoint(now, int64(m.ServiceStatus), s, metadata.AttributeWindowsServiceStartupMode(m.StartType))
	}

	return wss.mb.Emit(), errs.Combine()
}
