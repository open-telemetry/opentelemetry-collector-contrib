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

package dockerstatsreceiver

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval"
)

var _ component.MetricsReceiver = (*Receiver)(nil)
var _ interval.Runnable = (*Receiver)(nil)

const (
	defaultDockerAPIVersion         = 1.22
	minimalRequiredDockerAPIVersion = 1.22
)

type Receiver struct {
	config            *Config
	settings          component.ReceiverCreateSettings
	nextConsumer      consumer.Metrics
	client            *docker.Client
	runner            *interval.Runner
	runnerCtx         context.Context
	runnerCancel      context.CancelFunc
	successfullySetup bool
	transport         string
	obsrecv           *obsreport.Receiver
}

func NewReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	config *Config,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("could not determine receiver transport: %w", err)
	}

	receiver := Receiver{
		config:       config,
		nextConsumer: nextConsumer,
		settings:     set,
		transport:    parsed.Scheme,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              parsed.Scheme,
			ReceiverCreateSettings: set,
		}),
	}

	return &receiver, nil
}

func (r *Receiver) Start(ctx context.Context, host component.Host) error {
	dConfig, err := docker.NewConfig(r.config.Endpoint, r.config.Timeout, r.config.ExcludedImages, r.config.DockerAPIVersion)
	if err != nil {
		return err
	}

	r.client, err = docker.NewDockerClient(dConfig, r.settings.Logger)
	if err != nil {
		return err
	}

	r.runnerCtx, r.runnerCancel = context.WithCancel(context.Background())
	r.runner = interval.NewRunner(r.config.CollectionInterval, r)

	go func() {
		if err := r.runner.Start(); err != nil {
			host.ReportFatalError(err)
		}
	}()

	return nil
}

func (r *Receiver) Shutdown(ctx context.Context) error {
	r.runnerCancel()
	r.runner.Stop()
	return nil
}

func (r *Receiver) Setup() error {
	err := r.client.LoadContainerList(r.runnerCtx)
	if err != nil {
		return err
	}

	go r.client.ContainerEventLoop(r.runnerCtx)
	r.successfullySetup = true
	return nil
}

type result struct {
	md  pdata.Metrics
	err error
}

func (r *Receiver) Run() error {
	if !r.successfullySetup {
		return r.Setup()
	}

	ctx := r.obsrecv.StartMetricsOp(r.runnerCtx)

	containers := r.client.Containers()
	results := make(chan result, len(containers))

	wg := &sync.WaitGroup{}
	wg.Add(len(containers))
	for _, container := range containers {
		go func(c docker.Container) {
			defer wg.Done()
			statsJSON, err := r.client.FetchContainerStatsAsJSON(ctx, c)
			if err != nil {
				results <- result{pdata.NewMetrics(), err}
				return
			}

			md, err := ContainerStatsToMetrics(pdata.NewTimestampFromTime(time.Now()), statsJSON, c, r.config)
			if err != nil {
				r.settings.Logger.Error(
					"Could not convert docker containerStats for container id",
					zap.String("id", c.ID),
					zap.Error(err),
				)
			}

			results <- result{md, err}
		}(container)
	}

	wg.Wait()
	close(results)

	numPoints := 0
	var lastErr error
	for res := range results {
		var err error
		currentNumPoints := res.md.DataPointCount()
		if currentNumPoints != 0 {
			numPoints += currentNumPoints
			err = r.nextConsumer.ConsumeMetrics(ctx, res.md)
		} else {
			err = res.err
		}

		if err != nil {
			lastErr = err
		}
	}

	r.obsrecv.EndMetricsOp(ctx, typeStr, numPoints, lastErr)
	return nil
}
