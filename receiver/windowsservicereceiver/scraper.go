// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"golang.org/x/sync/errgroup"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
)

const hardCapWorkers = 64

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

func workerLimit() int {
	n := runtime.GOMAXPROCS(0) * 2
	if n < 4 {
		n = 4
	}
	if n > hardCapWorkers {
		n = hardCapWorkers
	}
	return n
}

func (ws *windowsServiceScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	ts := pcommon.NewTimestampFromTime(time.Now())

	names, err := ws.mgr.listServices()
	if err != nil {
		return ws.mb.Emit(), err
	}

	filtered := make([]string, 0, len(names))
	for _, n := range names {
		if ws.allowed(n) {
			filtered = append(filtered, n)
		}
	}
	if len(filtered) == 0 {
		return ws.mb.Emit(), nil
	}

	sem := make(chan struct{}, workerLimit())

	type svcResult struct {
		name      string
		val       int64
		startAttr metadata.AttributeStartupMode
	}
	type svcError struct {
		name string
		err  error
	}

	results := make(chan svcResult, len(filtered))
	errsCh := make(chan svcError, len(filtered))

	g, gctx := errgroup.WithContext(ctx)

	for _, name := range filtered {
		n := name
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
			case <-gctx.Done():
				return gctx.Err()
			}
			defer func() { <-sem }()

			svc, err := updateService(&ws.mgr, n)
			if err != nil {
				select {
				case errsCh <- svcError{n, err}:
				default:
					ws.logger.Debug("updateService failed", zap.String("service", n), zap.Error(err))
				}
				return nil
			}
			defer func() { _ = svc.close() }()

			if err := svc.updateStatus(); err != nil {
				select {
				case errsCh <- svcError{n, err}:
				default:
					ws.logger.Debug("updateStatus failed", zap.String("service", n), zap.Error(err))
				}
				return nil
			}
			if err := svc.updateConfig(); err != nil {
				select {
				case errsCh <- svcError{n, err}:
				default:
					ws.logger.Debug("updateConfig failed", zap.String("service", n), zap.Error(err))
				}
				return nil
			}

			val := int64(svc.status.State)
			if val < 1 || val > 7 {
				val = 0
			}

			select {
			case results <- svcResult{
				name:      n,
				val:       val,
				startAttr: mapStartTypeToAttr(svc.config.StartType),
			}:
				return nil
			case <-gctx.Done():
				return gctx.Err()
			}
		})
	}

	go func() {
		_ = g.Wait()
		close(results)
		close(errsCh)
	}()

	for r := range results {
		ws.mb.RecordWindowsServiceStatusDataPoint(ts, r.val, r.name, r.startAttr)
	}

	var aggErr error
	for e := range errsCh {
		aggErr = multierr.Append(aggErr, fmt.Errorf("%s: %w", e.name, e.err))
	}
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		aggErr = multierr.Append(aggErr, err)
	}

	return ws.mb.Emit(), aggErr
}
