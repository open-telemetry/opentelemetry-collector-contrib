// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper"

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

type SimplePrometheusScraper struct {
	Ctx                context.Context
	Settings           component.TelemetrySettings
	host               component.Host
	HostInfoProvider   HostInfoProvider
	PrometheusReceiver receiver.Metrics
	ScraperConfigs     *config.ScrapeConfig
	running            bool
}

type SimplePrometheusScraperOpts struct {
	Ctx               context.Context
	TelemetrySettings component.TelemetrySettings
	Consumer          consumer.Metrics
	Host              component.Host
	HostInfoProvider  HostInfoProvider
	ScraperConfigs    *config.ScrapeConfig
	Logger            *zap.Logger
}

type HostInfoProvider interface {
	GetClusterName() string
	GetInstanceID() string
	GetInstanceType() string
}

func NewSimplePrometheusScraper(opts SimplePrometheusScraperOpts) (*SimplePrometheusScraper, error) {
	if opts.Consumer == nil {
		return nil, errors.New("consumer cannot be nil")
	}
	if opts.Host == nil {
		return nil, errors.New("host cannot be nil")
	}
	if opts.HostInfoProvider == nil {
		return nil, errors.New("cluster name provider cannot be nil")
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &prometheusreceiver.PromConfig{
			ScrapeConfigs: []*config.ScrapeConfig{opts.ScraperConfigs},
		},
	}

	params := receiver.Settings{
		ID:                component.MustNewID(opts.ScraperConfigs.JobName),
		TelemetrySettings: opts.TelemetrySettings,
	}

	promFactory := prometheusreceiver.NewFactory()
	promReceiver, err := promFactory.CreateMetrics(opts.Ctx, params, &promConfig, opts.Consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus receiver: %w", err)
	}

	return &SimplePrometheusScraper{
		Ctx:                opts.Ctx,
		Settings:           opts.TelemetrySettings,
		host:               opts.Host,
		HostInfoProvider:   opts.HostInfoProvider,
		ScraperConfigs:     opts.ScraperConfigs,
		PrometheusReceiver: promReceiver,
	}, nil
}

func (ds *SimplePrometheusScraper) GetMetrics() []pmetric.Metrics {
	// This method will never return metrics because the metrics are collected by the scraper.
	// This method will ensure the scraper is running

	if !ds.running {
		ds.Settings.Logger.Info("The scraper is not running, starting up the scraper")
		err := ds.PrometheusReceiver.Start(ds.Ctx, ds.host)
		if err != nil {
			ds.Settings.Logger.Error("Unable to start PrometheusReceiver", zap.Error(err))
		}
		ds.running = err == nil
	}
	return nil
}

func (ds *SimplePrometheusScraper) Shutdown() {
	if ds.running {
		err := ds.PrometheusReceiver.Shutdown(ds.Ctx)
		if err != nil {
			ds.Settings.Logger.Error("Unable to shutdown PrometheusReceiver", zap.Error(err))
		}
		ds.running = false
	}
}
