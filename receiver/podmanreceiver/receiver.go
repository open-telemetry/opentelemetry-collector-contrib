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
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval"
)

var _ component.MetricsReceiver = (*receiver)(nil)
var _ interval.Runnable = (*receiver)(nil)

type receiver struct {
	config        *Config
	logger        *zap.Logger
	nextConsumer  consumer.Metrics
	clientFactory clientFactory

	client       client
	runner       *interval.Runner
	runnerCtx    context.Context
	runnerCancel context.CancelFunc

	obsrecv *obsreport.Receiver
}

func newReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	config *Config,
	nextConsumer consumer.Metrics,
	clientFactory clientFactory,
) (component.MetricsReceiver, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("could not determine receiver transport: %w", err)
	}

	if clientFactory == nil {
		clientFactory = newPodmanClient
	}

	return &receiver{
		config:        config,
		nextConsumer:  nextConsumer,
		clientFactory: clientFactory,
		logger:        set.Logger,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              parsed.Scheme,
			ReceiverCreateSettings: set,
		}),
	}, nil
}

func (r *receiver) Start(ctx context.Context, host component.Host) error {
	r.runnerCtx, r.runnerCancel = context.WithCancel(context.Background())
	r.runner = interval.NewRunner(r.config.CollectionInterval, r)
	go func() {
		if err := r.runner.Start(); err != nil {
			host.ReportFatalError(err)
		}
	}()
	return nil
}

func (r *receiver) Shutdown(ctx context.Context) error {
	r.runnerCancel()
	r.runner.Stop()
	return nil
}

func (r *receiver) Setup() error {
	c, err := r.clientFactory(r.logger, r.config)
	if err == nil {
		r.client = c
	}
	return err
}

func (r *receiver) Run() error {
	var numPoints int
	var err error
	ctx := r.obsrecv.StartMetricsOp(context.Background())
	defer func() {
		if err != nil {
			r.logger.Error("error fetching/processing metrics", zap.Error(err))
		}
		r.obsrecv.EndMetricsOp(ctx, typeStr, numPoints, err)
	}()

	stats, err := r.client.stats()
	if err != nil {
		// if we return an error, interval will stop the Run and never try again
		// so we never return from this functio and instead log errors and keep
		// retrying.
		r.logger.Error("error fetching stats", zap.Error(err))
		return nil
	}

	numPoints, err = r.consumeStats(ctx, stats)
	return nil
}

func (r *receiver) consumeStats(ctx context.Context, stats []containerStats) (int, error) {
	numPoints := 0
	var lastErr error

	for i := range stats {
		md := translateStatsToMetrics(&stats[i], time.Now())
		numPoints += md.DataPointCount()
		err := r.nextConsumer.ConsumeMetrics(ctx, md)
		if err != nil {
			lastErr = fmt.Errorf("failed to consume stats: %w", err)
			continue
		}
	}
	return numPoints, lastErr
}
