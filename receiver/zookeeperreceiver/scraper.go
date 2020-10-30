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

package zookeeperreceiver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/simple"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver/internal/metadata"
)

var zookeeperFormatRE = regexp.MustCompile(`(^zk_\w+)\s+([\w\.\-]+)`)

const (
	mntrCommand = "mntr"
)

type zookeeperMetricsScraper struct {
	logger *zap.Logger
	config *Config
	cancel context.CancelFunc
}

func newZookeeperMetricsScraper(logger *zap.Logger, config *Config) (*zookeeperMetricsScraper, error) {
	_, _, err := net.SplitHostPort(config.TCPAddr.Endpoint)
	if err != nil {
		return nil, err
	}

	if config.Timeout <= 0 {
		return nil, errors.New("timeout must be a positive duration")
	}

	return &zookeeperMetricsScraper{
		logger: logger,
		config: config,
	}, nil
}

func (z *zookeeperMetricsScraper) Initialize(_ context.Context) error {
	return nil
}

func (z *zookeeperMetricsScraper) Close(_ context.Context) error {
	z.cancel()
	return nil
}

func (z *zookeeperMetricsScraper) Scrape(ctx context.Context) (pdata.ResourceMetricsSlice, error) {
	var ctxWithTimeout context.Context
	ctxWithTimeout, z.cancel = context.WithTimeout(ctx, z.config.Timeout)

	conn, err := z.config.Dial()
	if err != nil {
		z.logger.Error("failed to establish connection",
			zap.String("endpoint", z.config.Endpoint),
			zap.Error(err),
		)
		return pdata.ResourceMetricsSlice{}, err
	}
	defer conn.Close()

	deadline, ok := ctxWithTimeout.Deadline()
	if ok {
		conn.SetDeadline(deadline)
	}

	return z.getResourceMetrics(conn), nil
}

func (z *zookeeperMetricsScraper) getResourceMetrics(conn net.Conn) pdata.ResourceMetricsSlice {
	scanner := sendCmd(conn, mntrCommand)
	metrics := simple.Metrics{
		Metrics:                    pdata.NewMetrics(),
		Timestamp:                  time.Now(),
		InstrumentationLibraryName: "otelcol/zookeeper",
		MetricFactoriesByName:      metadata.M.FactoriesByName(),
		ResourceAttributes:         map[string]string{},
	}

	for scanner.Scan() {
		line := scanner.Text()
		parts := zookeeperFormatRE.FindStringSubmatch(line)
		if len(parts) != 3 {
			z.logger.Warn("unexpected line in response",
				zap.String("command", mntrCommand),
				zap.String("line", line),
			)
			continue
		}

		switch parts[1] {
		case zkVersionKey:
			metrics.ResourceAttributes[zkVersionResourceLabel] = parts[2]
			continue
		case serverStateKey:
			metrics.ResourceAttributes[serverStateResourceLabel] = parts[2]
			continue
		default:
			// Skip metric if there is no descriptor associated with it.
			metricDescriptor := getOTLPMetricDescriptor(parts[1])
			if metricDescriptor.IsNil() {
				continue
			}

			int64Val, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				z.logger.Debug(
					fmt.Sprintf("non-integer value from %s", mntrCommand),
					zap.String("value", parts[2]),
				)
				continue
			}

			// Currently the receiver only deals with one metric type.
			switch metricDescriptor.DataType() {
			case pdata.MetricDataTypeIntGauge:
				metrics.AddGaugeDataPoint(metricDescriptor.Name(), int64Val)
			}

		}
	}
	return metrics.ResourceMetrics()
}
