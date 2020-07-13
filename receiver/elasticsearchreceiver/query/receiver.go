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

package query

import (
	"context"
	"encoding/json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/util"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.uber.org/zap"
)

type ESReceiver struct {
	logger *zap.Logger
	config *Config
	next   consumer.MetricsConsumer
	cancel context.CancelFunc
}

func NewESReceiver(
	logger *zap.Logger,
	config *Config,
	consumer consumer.MetricsConsumer,
) *ESReceiver {
	return &ESReceiver{
		logger: logger,
		config: config,
		next:   consumer,
	}
}

// Set up and kick off the interval runner.
func (e *ESReceiver) Start(ctx context.Context, host component.Host) error {
	httpClient, err := e.config.HTTPClientSettings.ToClient()
	if err != nil {
		return err
	}

	esClient := NewESQueryClient(e.config.Endpoint, httpClient)

	var rCtx context.Context
	rCtx, e.cancel = context.WithCancel(ctx)

	var reqBody ElasticsearchQueryBody
	if err = json.Unmarshal([]byte(e.config.ElasticsearchRequest), &reqBody); err != nil {
		return err
	}

	aggsMeta, err := reqBody.AggregationsMeta()
	if err != nil {
		return err
	}

	util.RunOnInterval(rCtx, func() {
		body, err := esClient.HTTPRequestFromConfig(e.config.Index, e.config.ElasticsearchRequest)

		if err != nil {
			e.logger.Error("Failed to make HTTP request", zap.Error(err))
			return
		}

		var resBody HTTPResponse
		if err := json.Unmarshal(body, &resBody); err != nil {
			e.logger.Error("Error processing HTTP response", zap.Error(err))
			return
		}

		mds := CollectMetrics(resBody, aggsMeta, map[string]string{
			"index": e.config.Index,
		}, e.logger)

		e.next.ConsumeMetrics(rCtx, pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{{Metrics: mds}}))
	}, e.config.CollectionInterval)

	return nil
}

func (e *ESReceiver) Shutdown(ctx context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}
	return nil
}
