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

	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/interval"
)

var _ interval.Runnable = (*redisRunnable)(nil)

// Runs intermittently, fetching info from Redis, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type redisRunnable struct {
	ctx             context.Context
	metricsConsumer consumer.MetricsConsumer
	redisSvc        *redisSvc
	redisMetrics    []*redisMetric
	logger          *zap.Logger
}

func newRedisRunnable(
	ctx context.Context,
	client client,
	metricsConsumer consumer.MetricsConsumer,
	logger *zap.Logger,
) *redisRunnable {
	return &redisRunnable{
		ctx:             ctx,
		redisSvc:        newRedisSvc(client),
		metricsConsumer: metricsConsumer,
		logger:          logger,
	}
}

// Builds a data structure of all of the keys, types, converters and such
// to later extract data from Redis.
func (r *redisRunnable) Setup() error {
	r.redisMetrics = getDefaultRedisMetrics()
	return nil
}

// Runs intermittently, querying Redis and building Metrics to send to the
// next consumer. First builds 'fixed' metrics (non-keyspace metrics) defined
// at startup time. Then builds 'keyspace' metrics if there are any keyspace
// lines returned by Redis. There should be one keyspace line per active Redis
// database, of which there can be 16.
func (r *redisRunnable) Run() error {
	const dataformat = "redis"
	const transport = "http" // todo verify this
	ctx := obsreport.StartMetricsReceiveOp(r.ctx, dataformat, transport)

	info, err := r.redisSvc.info()
	if err != nil {
		obsreport.EndMetricsReceiveOp(ctx, dataformat, 0, 0, err)
		return err
	}
	now := time.Now()
	metrics, warnings := info.buildFixedProtoMetrics(r.redisMetrics, now)
	if warnings != nil {
		r.logger.Warn(
			"errors parsing redis string",
			zap.Errors("parsing errors", warnings),
		)
	}

	keyspaceMetrics, warnings := info.buildKeyspaceProtoMetrics(now)
	metrics = append(metrics, keyspaceMetrics...)
	if warnings != nil {
		r.logger.Warn(
			"errors parsing keyspace string",
			zap.Errors("parsing errors", warnings),
		)
	}

	md := newMetricsData(metrics)

	err = r.metricsConsumer.ConsumeMetricsData(r.ctx, *md)
	if err != nil {
		obsreport.EndMetricsReceiveOp(ctx, dataformat, 0, 0, err)
	}

	numTimeSeries, numPoints := obsreport.CountMetricPoints(*md)
	obsreport.EndMetricsReceiveOp(ctx, dataformat, numPoints, numTimeSeries, err)

	return nil
}
