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

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
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
	logsConsumer     consumer.Logs

	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func newReceiver(
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

	return recv, err
}

func (r *receiver) registerMetricsConsumer(mc consumer.Metrics, set component.ReceiverCreateSettings) error {
	scrp, err := scraperhelper.NewScraper(typeStr, r.scrape)
	if err != nil {
		return err
	}
	r.metricsComponent, err = scraperhelper.NewScraperControllerReceiver(&r.config.ScraperControllerSettings, set, mc, scraperhelper.AddScraper(scrp))
	return err
}

func (r *receiver) registerLogsConsumer(lc consumer.Logs) {
	r.logsConsumer = lc
}

func (r *receiver) Start(ctx context.Context, host component.Host) error {
	c, err := r.clientFactory(r.set.Logger, r.config)
	if err != nil {
		return fmt.Errorf("error setting up client: %w", err)
	}
	r.client = c

	// Check for logs pipeline
	if r.logsConsumer == nil {
		r.set.Logger.Debug("logs receiver is not set")
	} else {
		eventBackoff := backoff.NewExponentialBackOff()
		eventBackoff.InitialInterval = 2 * time.Second
		eventBackoff.MaxInterval = 10 * time.Minute
		eventBackoff.Multiplier = 2
		eventBackoff.MaxElapsedTime = 0

		ctx, r.cancel = context.WithCancel(ctx)
		backoffContext := backoff.WithContext(eventBackoff, ctx)

		r.wg = &sync.WaitGroup{}
		r.wg.Add(1)
		go func() {
			// Retry if any errors occur while getting the events.
			defer r.wg.Done()
			errorWhileRetry := backoff.Retry(func() error {
				err = r.handleEvents(ctx, eventBackoff)
				return fmt.Errorf("error fetching/processing events: %w", err)
			}, backoffContext)
			if errorWhileRetry != nil {
				// context will be cancelled only in the case when shutdown will be called. So it's a graceful return.
				if errors.Is(errorWhileRetry, context.Canceled) {
					return
				}
				r.set.Logger.Error("retry failed", zap.Error(errorWhileRetry))
				return
			}
		}()
	}

	// Check for metrics pipeline
	if r.metricsComponent == nil {
		r.set.Logger.Debug("metrics receiver is not set")
	} else {
		go func() {
			err = r.metricsComponent.Start(ctx, host)
			if err != nil {
				r.set.Logger.Error("error starting metrics receiver", zap.Error(err))
			}
		}()
	}
	return nil
}

func (r *receiver) Shutdown(ctx context.Context) error {
	if r.metricsComponent != nil {
		err := r.metricsComponent.Shutdown(ctx)
		if err != nil {
			return err
		}
	}
	if r.logsConsumer != nil {
		r.cancel()
		r.wg.Wait()
	}
	return nil

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

func (r *receiver) handleEvents(ctx context.Context, eventBackoff *backoff.ExponentialBackOff) error {
	// Fetch the response from the endpoint
	response, err := r.client.getEventsResponse(ctx)
	if err != nil {
		return err
	}

	dec := json.NewDecoder(response.Body)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Translate the response to the events format.
			decodedEvent, err := decodeEvents(dec)
			if err != nil {
				if err == io.EOF {
					break
				}
				// context will be cancelled only in the case when shutdown will be called. So returning gracefully.
				if errors.Is(err, context.Canceled) {
					return nil
				}
				if err = response.Body.Close(); err != nil {
					return err
				}
				return err
			}

			// Translate the events into the pdata.logs format
			ld, err := translateEventsToLogs(r.set.Logger, decodedEvent)
			if err != nil {
				return err
			}

			err = r.logsConsumer.ConsumeLogs(ctx, ld)
			if err != nil {
				return err
			}
			eventBackoff.Reset()
		}
	}
}
