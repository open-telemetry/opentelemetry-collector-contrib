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

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval"
)

var _ interval.Runnable = (*redisRunnable)(nil)

const transport = "http" // todo verify this

// Runs intermittently, fetching info from Redis, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type redisRunnable struct {
	id              config.ComponentID
	ctx             context.Context
	metricsConsumer consumer.Metrics
	redisSvc        *redisSvc
	redisMetrics    []*redisMetric
	logger          *zap.Logger
	timeBundle      *timeBundle
	obsrecv         *obsreport.Receiver
}

func newRedisRunnable(
	ctx context.Context,
	id config.ComponentID,
	client client,
	metricsConsumer consumer.Metrics,
	logger *zap.Logger,
) *redisRunnable {
	return &redisRunnable{
		id:              id,
		ctx:             ctx,
		redisSvc:        newRedisSvc(client),
		metricsConsumer: metricsConsumer,
		logger:          logger,
		obsrecv:         obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: id, Transport: transport}),
	}
}

// Setup builds a data structure of all of the keys, types, converters and such to
// later extract data from Redis.
func (r *redisRunnable) Setup() error {
	r.redisMetrics = getDefaultRedisMetrics()
	return nil
}

// Run is called periodically, querying Redis and building Metrics to send to
// the next consumer. First builds 'fixed' metrics (non-keyspace metrics)
// defined at startup time. Then builds 'keyspace' metrics if there are any
// keyspace lines returned by Redis. There should be one keyspace line per
// active Redis database, of which there can be 16.
func (r *redisRunnable) Run() error {
	const dataFormat = "redis"
	ctx := r.obsrecv.StartMetricsOp(r.ctx)

	inf, err := r.redisSvc.info()
	if err != nil {
		r.obsrecv.EndMetricsOp(ctx, dataFormat, 0, err)
		return nil
	}

	uptime, err := inf.getUptimeInSeconds()
	if err != nil {
		r.obsrecv.EndMetricsOp(ctx, dataFormat, 0, err)
		return nil
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
		r.logger.Warn(
			"errors parsing redis string",
			zap.Errors("parsing errors", warnings),
		)
	}

	keyspaceMS, warnings := inf.buildKeyspaceMetrics(r.timeBundle)
	if warnings != nil {
		r.logger.Warn(
			"errors parsing keyspace string",
			zap.Errors("parsing errors", warnings),
		)
	}
	keyspaceMS.MoveAndAppendTo(ilm.Metrics())

	numPoints := pdm.DataPointCount()
	err = r.metricsConsumer.ConsumeMetrics(r.ctx, pdm)
	r.obsrecv.EndMetricsOp(ctx, dataFormat, numPoints, err)

	return nil
}
