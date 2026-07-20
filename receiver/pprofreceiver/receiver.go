// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.opentelemetry.io/collector/scraper/scraperhelper/xscraperhelper"
	"go.opentelemetry.io/collector/scraper/xscraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

type pprofReceiver struct {
	subComponents []component.Component
}

var _ xreceiver.Profiles = (*pprofReceiver)(nil)

func (r *pprofReceiver) Start(ctx context.Context, host component.Host) error {
	for i, c := range r.subComponents {
		if err := c.Start(ctx, host); err != nil {
			// Roll back any already-started sub-components so we don't leak
			// goroutines or hold a listening port on partial start failure.
			for j := i - 1; j >= 0; j-- {
				err = errors.Join(err, r.subComponents[j].Shutdown(ctx))
			}
			return err
		}
	}
	return nil
}

func (r *pprofReceiver) Shutdown(ctx context.Context) error {
	var errs error
	for _, c := range r.subComponents {
		if c == nil {
			continue
		}
		errs = errors.Join(errs, c.Shutdown(ctx))
	}
	return errs
}

func newReceiver(cfg *Config, settings receiver.Settings, consumer xconsumer.Profiles) (xreceiver.Profiles, error) {
	r := &pprofReceiver{}

	if cfg.Remote.HasValue() {
		remoteCfg := cfg.Remote.Get()
		sub, err := newScraperController(settings, consumer, &remoteCfg.ControllerConfig, func(set scraper.Settings) (xscraper.Profiles, error) {
			return &internal.HTTPClientScraper{
				ClientConfig: remoteCfg.ClientConfig,
				Settings:     set.TelemetrySettings,
				BuildInfo:    settings.BuildInfo,
			}, nil
		})
		if err != nil {
			return nil, err
		}
		r.subComponents = append(r.subComponents, sub)
	}

	if cfg.File.HasValue() {
		fileCfg := cfg.File.Get()
		sub, err := newScraperController(settings, consumer, &fileCfg.ControllerConfig, func(set scraper.Settings) (xscraper.Profiles, error) {
			return xscraper.NewProfiles((&internal.FileScraper{
				Include:   fileCfg.Include,
				Logger:    set.Logger,
				BuildInfo: settings.BuildInfo,
			}).Scrape)
		})
		if err != nil {
			return nil, err
		}
		r.subComponents = append(r.subComponents, sub)
	}

	if cfg.Self.HasValue() {
		selfCfg := cfg.Self.Get()
		sub, err := newScraperController(settings, consumer, &selfCfg.ControllerConfig, func(_ scraper.Settings) (xscraper.Profiles, error) {
			return &internal.SelfScraper{
				BlockProfileFraction: selfCfg.BlockProfileFraction,
				MutexProfileFraction: selfCfg.MutexProfileFraction,
				BuildInfo:            settings.BuildInfo,
			}, nil
		})
		if err != nil {
			return nil, err
		}
		r.subComponents = append(r.subComponents, sub)
	}

	if cfg.Server.HasValue() {
		serverCfg := cfg.Server.Get()
		r.subComponents = append(r.subComponents, &internal.HTTPServer{
			ServerConfig: serverCfg.ServerConfig,
			Consumer:     consumer,
			Settings:     settings,
		})
	}

	return r, nil
}

func newScraperController(
	settings receiver.Settings,
	consumer xconsumer.Profiles,
	controllerCfg *scraperhelper.ControllerConfig,
	scraperFn func(scraper.Settings) (xscraper.Profiles, error),
) (xreceiver.Profiles, error) {
	scraperFactory := xscraper.NewFactory(
		metadata.Type,
		func() component.Config { return &struct{}{} },
		xscraper.WithProfiles(func(_ context.Context, set scraper.Settings, _ component.Config) (xscraper.Profiles, error) {
			return scraperFn(set)
		}, metadata.ProfilesStability),
	)
	return xscraperhelper.NewProfilesController(
		controllerCfg,
		settings,
		consumer,
		xscraperhelper.AddFactoryWithConfig(scraperFactory, nil),
	)
}
