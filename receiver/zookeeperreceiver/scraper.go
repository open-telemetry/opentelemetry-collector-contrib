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
	"go.uber.org/zap"
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

	rawStats, serverState := z.getRawMNTRStats(conn)

	metricsSlice := pdata.NewMetricSlice()
	metricsSlice.Resize(len(rawStats))

	now := timeToUnixNano(time.Now())

	var idx int
	for _, rawStat := range rawStats {
		metricDescriptor := getOTLPMetricDescriptor(rawStat.metric)
		initializeMetric(metricsSlice.At(idx), metricDescriptor, now, rawStat.val)
		idx++
	}

	return resourceMetricSlice(serverState, metricsSlice), nil
}

func resourceMetricSlice(serverState serverState, metricsSlice pdata.MetricSlice) pdata.ResourceMetricsSlice {
	resourceMetricSlice := pdata.NewResourceMetricsSlice()
	resourceMetricSlice.Resize(1)
	rm := resourceMetricSlice.At(0)
	rm.Resource().InitEmpty()
	rm.Resource().Attributes().Insert(serverStateResourceLabel, pdata.NewAttributeValueString(string(serverState)))

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Resize(1)
	ilm := ilms.At(0)
	metricsSlice.MoveAndAppendTo(ilm.Metrics())

	return resourceMetricSlice
}

type rawStat struct {
	metric string
	val    int64
}

type serverState string

func (z *zookeeperMetricsScraper) getRawMNTRStats(conn net.Conn) ([]rawStat, serverState) {
	scanner := sendCmd(conn, mntrCommand)
	rawStats := make([]rawStat, 0, maxMetricsLen)
	var state serverState

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

		// Skip metric if there is no descriptor associated with it.
		if getOTLPMetricDescriptor(parts[1]).IsNil() {
			if serverStateKey == parts[1] {
				state = serverState(parts[2])
			}
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
		rawStats = append(rawStats, rawStat{metric: parts[1], val: int64Val})
	}
	return rawStats, state
}
