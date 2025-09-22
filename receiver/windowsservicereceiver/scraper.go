// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
)

type windowsServiceScraper struct {
	logger     *zap.Logger
	cfg        *Config
	mb         *metadata.MetricsBuilder
	mgr        serviceManager
	includeSet map[string]struct{}
	excludeSet map[string]struct{}
}

const defaultScrapeConcurrency = 32

func newWindowsServiceScraper(settings receiver.Settings, cfg *Config, mb *metadata.MetricsBuilder) *windowsServiceScraper {
	ws := &windowsServiceScraper{
		logger: settings.Logger,
		cfg:    cfg,
		mb:     mb,
		mgr:    serviceManager{},
	}
	if len(cfg.IncludeServices) > 0 {
		ws.includeSet = make(map[string]struct{}, len(cfg.IncludeServices))
		for _, n := range cfg.IncludeServices {
			ws.includeSet[n] = struct{}{}
		}
	}
	if len(cfg.ExcludeServices) > 0 {
		ws.excludeSet = make(map[string]struct{}, len(cfg.ExcludeServices))
		for _, n := range cfg.ExcludeServices {
			ws.excludeSet[n] = struct{}{}
		}
	}
	return ws
}

func mapStartTypeToAttr(st StartType) metadata.AttributeStartupMode {
	switch st {
	case StartBoot:
		return metadata.AttributeStartupModeBootStart
	case StartSystem:
		return metadata.AttributeStartupModeSystemStart
	case StartAutomatic:
		return metadata.AttributeStartupModeAutoStart
	case StartManual:
		return metadata.AttributeStartupModeDemandStart
	case StartDisabled:
		return metadata.AttributeStartupModeDisabled
	default:
		return metadata.AttributeStartupModeDemandStart
	}
}

func (ws *windowsServiceScraper) start(_ context.Context, _ component.Host) error {
	return ws.mgr.connect()
}

func (ws *windowsServiceScraper) shutdown(_ context.Context) error {
	return ws.mgr.disconnect()
}

func (ws *windowsServiceScraper) allowed(name string) bool {
	if len(ws.includeSet) > 0 {
		if _, ok := ws.includeSet[name]; !ok {
			return false
		}
	}
	if _, banned := ws.excludeSet[name]; banned {
		return false
	}
	return true
}

func (ws *windowsServiceScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	ts := pcommon.NewTimestampFromTime(time.Now())

	names, err := ws.mgr.listServices()
	if err != nil {
		return ws.mb.Emit(), err
	}

	type result struct {
		name      string
		val       int64
		startAttr metadata.AttributeStartupMode
		err       error
	}

	results := make(chan result, len(names))
	sem := make(chan struct{}, defaultScrapeConcurrency)
	var wg sync.WaitGroup

	for _, name := range names {
		if !ws.allowed(name) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(n string) {
			defer wg.Done()
			defer func() { <-sem }()

			svc, err := getService(&ws.mgr, n)
			if err != nil {
				results <- result{err: err}
				return
			}
			defer func() { _ = svc.close() }()

			if err := svc.updateStatus(); err != nil {
				results <- result{err: err}
				return
			}
			if err := svc.updateConfig(); err != nil {
				results <- result{err: err}
				return
			}

			val := int64(svc.status.State)
			startAttr := mapStartTypeToAttr(svc.config.StartType)

			results <- result{name: n, val: val, startAttr: startAttr}
		}(name)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var scrapeErr error
	for r := range results {
		if r.err != nil {
			scrapeErr = multierr.Append(scrapeErr, r.err)
			continue
		}
		ws.mb.RecordWindowsServiceStatusDataPoint(ts, r.val, r.name, r.startAttr)
	}

	return ws.mb.Emit(), scrapeErr
}
