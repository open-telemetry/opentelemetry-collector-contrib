// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver/internal/redfish"
)

// scraperClient is a struct containing the RedfishClient
// and the resources it needs to collect.
type scraperClient struct {
	*redfish.Client
	ResourceSet map[Resource]bool
}

// redfishScraper is a scraper responsible for scraping redfish metrics using multiple
// scraperClients to scrape each redfish server in the given otel config.
type redfishScraper struct {
	clients  []*scraperClient
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
	logger   *zap.Logger
}

func newScraper(conf *Config, settings receiver.Settings) *redfishScraper {
	return &redfishScraper{
		cfg:      conf,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		clients:  make([]*scraperClient, 0, len(conf.Servers)),
		logger:   settings.Logger,
	}
}

// start is a method to initialize our redfishScraper scraperClients
func (s *redfishScraper) start(_ context.Context, _ component.Host) error {
	s.logger.Info("starting redfish scraper")

	// create redfish clients
	for i := range s.cfg.Servers {
		server := s.cfg.Servers[i]

		opts := make([]redfish.ClientOption, 0)
		opts = append(opts, redfish.WithInsecure(server.Insecure))

		if server.Redfish.Version != "" {
			opts = append(opts, redfish.WithRedfishVersion(server.Redfish.Version))
		}

		if timeout, err := time.ParseDuration(server.Timeout); err == nil {
			opts = append(opts, redfish.WithClientTimeout(timeout))
		}

		// create redfish client
		client, err := redfish.NewClient(
			server.ComputerSystemID,
			server.BaseURL,
			server.User,
			server.Pwd,
			opts...,
		)
		if err != nil {
			return err
		}

		// create resource set for each scraper client
		resourceSet := make(map[Resource]bool, len(server.Resources))
		for _, rc := range server.Resources {
			resourceSet[rc] = true
		}

		// append new scraper client
		s.clients = append(
			s.clients,
			&scraperClient{client, resourceSet},
		)
	}

	return nil
}

// scrape is a method invoked periodically to scrape all server redfish apis
// and add their metrics to a metrics buffer
func (s *redfishScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	s.logger.Info("scraping redfish...")

	errs := &scrapererror.ScrapeErrors{}
	for _, client := range s.clients {
		baseURL := client.BaseURL.String()

		compSys, err := client.GetComputerSystem()
		if err != nil {
			errs.Add(err)
			continue
		}

		// only record computer system metrics if it exists in the scraperClient's resourceSet
		if _, exists := client.ResourceSet[ComputerSystemResource]; exists {
			s.recordComputerSystem(compSys)
		}

		for _, link := range compSys.Links.Chassis {
			// Chassis metrics depend on ComputerSystem data
			chassis, err := client.GetChassis(link.Ref)
			if err != nil {
				errs.Add(err)
				continue
			}

			// only record chassis metrics if it exists in the scraperClient's resourceSet
			if _, exists := client.ResourceSet[ChassisResource]; exists {
				s.recordChassis(chassis)
			}

			// only scrape Fans and Temperatures if they exist in the scraperClient's resourceSet
			if client.ResourceSet[FansResource] || client.ResourceSet[TemperaturesResource] {
				thermal, err := client.GetThermal(chassis.Thermal.Ref)
				if err != nil {
					errs.Add(err)
					continue
				}

				// only record Fans metrics if it exists in the scraperClient's resourceSet
				if client.ResourceSet[FansResource] {
					s.recordFans(chassis.ID, thermal.Fans)
				}

				// only record Temperatures metrics if it exists in the scraperClient's resourceSet
				if client.ResourceSet[TemperaturesResource] {
					s.recordTemperatures(chassis.ID, thermal.Temperatures)
				}
			}
		}

		// Always present - resource attributes
		rb := s.mb.NewResourceBuilder()
		rb.SetBaseURL(baseURL)
		rb.SetSystemHostName(compSys.HostName)

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return s.mb.Emit(), errs.Combine()
}
