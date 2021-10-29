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

//go:build !windows
// +build !windows

package podmanreceiver

import (
	"context"
	"fmt"
	"go.opentelemetry.io/collector/obsreport"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

type receiver struct {
	config        *Config
	set           component.ReceiverCreateSettings
	clientFactory clientFactory
	client        client

	metricsComponent component.MetricsReceiver
	obsrecv *obsreport.Receiver
	logsConsumer	consumer.Logs
	metricsConsumer	consumer.Metrics
}

func newMetricsReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	config *Config,
	nextConsumer consumer.Metrics,
	clientFactory clientFactory,
) (*receiver, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	if clientFactory == nil {
		clientFactory = newPodmanClient
	}

	recv := &receiver{
		config:        config,
		clientFactory: clientFactory,
		metricsConsumer: nextConsumer,
		set:           set,
	}

	scrp, err := scraperhelper.NewScraper(typeStr, recv.scrape, scraperhelper.WithStart(recv.start))
	if err != nil {
		return nil, err
	}
	recv.metricsComponent, err = scraperhelper.NewScraperControllerReceiver(&recv.config.ScraperControllerSettings, set, nextConsumer, scraperhelper.AddScraper(scrp))
	return recv, err
}

func newLogsReceiver (
	_ context.Context,
	set component.ReceiverCreateSettings,
	config *Config,
	clientFactory clientFactory,
) (*receiver, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	if clientFactory == nil {
		clientFactory = newPodmanClient
	}
	recv := &receiver{
		config:        config,
		clientFactory: clientFactory,
		set:           set,
	}
	return recv, nil
}

func (r *receiver) RegisterLogsConsumer(lc consumer.Logs) {
	r.logsConsumer = lc
}


func (r *receiver) Start(ctx context.Context, host component.Host) error {
	if r.logsConsumer != nil {
		fmt.Println("Logs started")
		go func() {
			r.handleEvents()
		}()
	}
	if r.metricsConsumer != nil {
		fmt.Println("Metrics Started")
		go func() {
			err := r.metricsComponent.Start(ctx, host)
			if err != nil {
				return
			}
		}()
	}
	return nil
}

func (r *receiver) Shutdown(ctx context.Context) error {
	if r.metricsConsumer != nil {
		err := r.metricsComponent.Shutdown(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *receiver) start(context.Context, component.Host) error {
	c, err := r.clientFactory(r.set.Logger, r.config)
	if err == nil {
		r.client = c
	}
	return err
}

func (r *receiver) scrape(context.Context) (pdata.Metrics, error) {
	var err error

	stats, err := r.client.stats()
	if err != nil {
		r.set.Logger.Error("error fetching stats", zap.Error(err))
		return pdata.Metrics{}, err
	}

	md := pdata.NewMetrics()
	for i := range stats {
		translateStatsToMetrics(&stats[i], time.Now(), md.ResourceMetrics().AppendEmpty())
	}
	return md, nil
}

func (r *receiver)handleEvents() error {
	var err error
	c, err := r.clientFactory(r.set.Logger, r.config)
	if err == nil {
		r.client = c
	}
	if err != nil {
		r.set.Logger.Error("error fetching/processing events", zap.Error(err))
	}

	events, _ := r.client.events()
	if err != nil {
		r.set.Logger.Error("error fetching stats", zap.Error(err))
	}
	for event := range events {
		fmt.Println(event.Message)
		// Convert events to pdata.logs
	}
	return nil
}