// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package redisreceiver

import (
	"context"
	"time"

	"github.com/go-redis/redis/v7"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

// Runs intermittently, fetching info from Redis, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type redisScraper struct {
	redisSvc     *redisSvc
	redisMetrics []*redisMetric
	settings     component.ReceiverCreateSettings
	timeBundle   *timeBundle
}

func newRedisScraper(cfg Config, settings component.ReceiverCreateSettings, passedClient client) (scraperhelper.Scraper, error) {
	opts := &redis.Options{
		Addr:     cfg.Endpoint,
		Password: cfg.Password,
	}

	var err error
	if opts.TLSConfig, err = cfg.TLS.LoadTLSConfig(); err != nil {
		return nil, err
	}

	var clnt client
	if passedClient == nil {
		clnt = newRedisClient(opts)
	} else {
		clnt = passedClient
	}

	rs := &redisScraper{
		redisSvc:     newRedisSvc(clnt),
		redisMetrics: getDefaultRedisMetrics(),
		settings:     settings,
	}
	return scraperhelper.NewScraper(typeStr, rs.Scrape)
}

// Scrape is called periodically, querying Redis and building Metrics to send to
// the next consumer. First builds 'fixed' metrics (non-keyspace metrics)
// defined at startup time. Then builds 'keyspace' metrics if there are any
// keyspace lines returned by Redis. There should be one keyspace line per
// active Redis database, of which there can be 16.
func (r *redisScraper) Scrape(context.Context) (pdata.Metrics, error) {
	inf, err := r.redisSvc.info()
	if err != nil {
		return pdata.Metrics{}, err
	}

	uptime, err := inf.getUptimeInSeconds()
	if err != nil {
		return pdata.Metrics{}, err
	}

	if r.timeBundle == nil {
		r.timeBundle = newTimeBundle(time.Now(), uptime)
	} else {
		r.timeBundle.update(time.Now(), uptime)
	}

	pdm := pdata.NewMetrics()
	rm := pdm.ResourceMetrics().AppendEmpty()
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/" + typeStr)
	fixedMS, warnings := inf.buildFixedMetrics(r.redisMetrics, r.timeBundle)
	fixedMS.MoveAndAppendTo(ilm.Metrics())
	if warnings != nil {
		r.settings.Logger.Warn(
			"errors parsing redis string",
			zap.Errors("parsing errors", warnings),
		)
	}

	keyspaceMS, warnings := inf.buildKeyspaceMetrics(r.timeBundle)
	if warnings != nil {
		r.settings.Logger.Warn(
			"errors parsing keyspace string",
			zap.Errors("parsing errors", warnings),
		)
	}
	keyspaceMS.MoveAndAppendTo(ilm.Metrics())

	return pdm, nil
}
