// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
)

type windowsServiceScraper struct {
	logger     *zap.Logger
	cfg        *Config
	mb         *metadata.MetricsBuilder
	mgr        serviceManager
	includeSet map[string]struct{}
	excludeSet map[string]struct{}

	disabled bool
}

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
	if err := ws.mgr.connect(); err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			ws.logger.Warn("windowsservicereceiver: access denied to Service Control Manager; metrics will not be collected", zap.Error(err))
			ws.disabled = true
			return nil
		}
		return err
	}
	return nil
}

func (ws *windowsServiceScraper) shutdown(_ context.Context) error {
	if ws.disabled {
		return nil
	}
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
	if ws.disabled {
		return ws.mb.Emit(), nil
	}

	ts := pcommon.NewTimestampFromTime(time.Now())

	names, err := ws.mgr.listServices()
	if err != nil {
		return ws.mb.Emit(), err
	}

	var scrapeErr scrapererror.ScrapeErrors

	for _, name := range names {
		if !ws.allowed(name) {
			continue
		}

		svc, err := updateService(&ws.mgr, name)
		if err != nil {
			scrapeErr.AddPartial(1, fmt.Errorf("updateService failed for %v: %w", name, err))
			continue
		}

		if err := svc.updateStatus(); err != nil {
			_ = svc.close()
			scrapeErr.AddPartial(1, fmt.Errorf("updateStatus failed for %v: %w", name, err))
			continue
		}
		if err := svc.updateConfig(); err != nil {
			_ = svc.close()
			scrapeErr.AddPartial(1, fmt.Errorf("updateConfig failed for %v: %w", name, err))
			continue
		}

		val := int64(svc.status.State)
		startAttr := mapStartTypeToAttr(svc.config.StartType)

		ws.mb.RecordWindowsServiceStatusDataPoint(ts, val, name, startAttr)

		if err := svc.close(); err != nil {
			scrapeErr.AddPartial(1, fmt.Errorf("failed to close service %v: %w", name, err))
		}
	}

	return ws.mb.Emit(), scrapeErr.Combine()
}
