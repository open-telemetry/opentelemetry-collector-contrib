// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
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

func mapStateToMetricValue(st State) int {
	switch st {
	case StateStopped:
		return 1
	case StateStartPending:
		return 2
	case StateStopPending:
		return 3
	case StateRunning:
		return 4
	case StateContinuePending:
		return 5
	case StatePausePending:
		return 6
	case StatePaused:
		return 7
	default:
		return 1
	}
}

func mapStartTypeToAttr(st StartType, _ bool) metadata.AttributeStartupMode {
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

	for _, name := range names {
		if !ws.allowed(name) {
			continue
		}

		svc, err := getService(&ws.mgr, name)
		if err != nil {
			continue
		}

		if err := svc.getStatus(); err != nil {
			_ = svc.close()
			continue
		}
		if err := svc.getConfig(); err != nil {
			_ = svc.close()
			continue
		}

		val := int64(mapStateToMetricValue(State(svc.status.State)))
		startAttr := mapStartTypeToAttr(svc.config.StartType, svc.config.DelayedAutoStart)

		ws.mb.RecordWindowsServiceStatusDataPoint(ts, val, name, startAttr)

		_ = svc.close()
	}

	return ws.mb.Emit(), nil
}
