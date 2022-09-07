// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"sync"
	"time"
)

type SNMPReceiver struct {
	_            context.Context
	config       *internal.Config
	settings     component.ReceiverCreateSettings
	logger       *zap.Logger
	snmpWrapper  *internal.GosnmpWrapper
	nextConsumer consumer.Metrics
}

func (receiver *SNMPReceiver) Start(ctx context.Context, host component.Host) error {
	initAttributes(receiver.config)
}

func initAttributes(config *internal.Config) {

}

func (receiver *SNMPReceiver) Shutdown(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func NewSNMPReceiver(
	_ context.Context,
	settings component.ReceiverCreateSettings,
	config *internal.Config,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	goSNMPWrapper, err := internal.NewWrapper(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SNMP receiver config %w", err)
	}

	return &SNMPReceiver{
		config:       config,
		settings:     settings,
		snmpWrapper:  &goSNMPWrapper,
		logger:       settings.Logger,
		nextConsumer: nextConsumer,
	}, nil

	//scrp, err := scraperhelper.NewScraper(typeStr, receiver.scrape, scraperhelper.WithStart(receiver.start))
	//if err != nil {
	//	return nil, err
	//}
	//return scraperhelper.NewScraperControllerReceiver(&receiver.config.ScraperControllerSettings, settings, nextConsumer, scraperhelper.AddScraper(scrp))
}

func (receiver *SNMPReceiver) start(ctx context.Context, _ component.Host) error {

	//receiver.snmpWrapper, err = docker.(dConfig, receiver.settings.Logger)
	//if err != nil {
	//	return err
	//}
	//
	//if err = receiver.client.LoadContainerList(ctx); err != nil {
	//	return err
	//}
	//
	//go receiver.client.ContainerEventLoop(ctx)
	return nil
}

type result struct {
	md  pdata.Metrics
	err error
}

func (r *receiver) scrape(ctx context.Context) (pdata.Metrics, error) {
	containers := r.client.Containers()
	results := make(chan result, len(containers))

	wg := &sync.WaitGroup{}
	wg.Add(len(containers))
	for _, container := range containers {
		go func(c docker.Container) {
			defer wg.Done()
			statsJSON, err := r.client.FetchContainerStatsAsJSON(ctx, c)
			if err != nil {
				results <- result{md: pdata.Metrics{}, err: err}
				return
			}

			results <- result{
				md:  ContainerStatsToMetrics(pdata.NewTimestampFromTime(time.Now()), statsJSON, c, r.config),
				err: nil}
		}(container)
	}

	wg.Wait()
	close(results)

	var errs error
	md := pdata.NewMetrics()
	for res := range results {
		if res.err != nil {
			// Don't know the number of failed metrics, but one container fetch is a partial error.
			errs = multierr.Append(errs, scrapererror.NewPartialScrapeError(res.err, 0))
			continue
		}
		res.md.ResourceMetrics().CopyTo(md.ResourceMetrics())
	}

	return md, errs
}
