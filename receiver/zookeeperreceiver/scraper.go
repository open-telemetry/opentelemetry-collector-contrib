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
	"bufio"
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

	// For mocking.
	closeConnection       func(net.Conn) error
	setConnectionDeadline func(net.Conn, time.Time) error
	sendCmd               func(net.Conn, string) (*bufio.Scanner, error)
}

func (z *zookeeperMetricsScraper) Name() string {
	return typeStr
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
		logger:                logger,
		config:                config,
		closeConnection:       closeConnection,
		setConnectionDeadline: setConnectionDeadline,
		sendCmd:               sendCmd,
	}, nil
}

func (z *zookeeperMetricsScraper) shutdown(_ context.Context) error {
	z.cancel()
	return nil
}

func (z *zookeeperMetricsScraper) scrape(ctx context.Context) (pdata.ResourceMetricsSlice, error) {
	var ctxWithTimeout context.Context
	ctxWithTimeout, z.cancel = context.WithTimeout(ctx, z.config.Timeout)

	conn, err := z.config.Dial()
	if err != nil {
		z.logger.Error("failed to establish connection",
			zap.String("endpoint", z.config.Endpoint),
			zap.Error(err),
		)
		return pdata.NewResourceMetricsSlice(), err
	}
	defer func() {
		if closeErr := z.closeConnection(conn); closeErr != nil {
			z.logger.Warn("failed to shutdown connection", zap.Error(closeErr))
		}
	}()

	deadline, ok := ctxWithTimeout.Deadline()
	if ok {
		if err := z.setConnectionDeadline(conn, deadline); err != nil {
			z.logger.Warn("failed to set deadline on connection", zap.Error(err))
		}
	}

	return z.getResourceMetrics(conn)
}

type stat struct {
	metric pdata.Metric
	val    int64
}

func (z *zookeeperMetricsScraper) getResourceMetrics(conn net.Conn) (pdata.ResourceMetricsSlice, error) {
	scanner, err := z.sendCmd(conn, mntrCommand)
	if err != nil {
		z.logger.Error("failed to send command",
			zap.Error(err),
			zap.String("command", mntrCommand),
		)
		return pdata.NewResourceMetricsSlice(), err
	}

	stats, attributes := z.getMetricsAndAttributes(scanner)
	metrics := simple.Metrics{
		Metrics:                    pdata.NewMetrics(),
		Timestamp:                  time.Now(),
		InstrumentationLibraryName: "otelcol/zookeeper",
		MetricFactoriesByName:      metadata.M.FactoriesByName(),
		ResourceAttributes:         attributes,
	}

	for _, stat := range stats {
		// Currently the receiver only deals with one metric type.
		switch stat.metric.DataType() {
		case pdata.MetricDataTypeIntGauge:
			metrics.AddGaugeDataPoint(stat.metric.Name(), stat.val)
		case pdata.MetricDataTypeIntSum:
			metrics.AddSumDataPoint(stat.metric.Name(), stat.val)
		}
	}
	return metrics.ResourceMetrics(), nil
}

func (z *zookeeperMetricsScraper) getMetricsAndAttributes(scanner *bufio.Scanner) ([]stat, map[string]string) {
	attributes := make(map[string]string, 2)
	stats := make([]stat, 0, metricsLen)
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

		metricKey := parts[1]
		metricValue := parts[2]
		switch metricKey {
		case zkVersionKey:
			attributes[metadata.Labels.ZkVersion] = metricValue
			continue
		case serverStateKey:
			attributes[metadata.Labels.ServerState] = metricValue
			continue
		default:
			// Skip metric if there is no descriptor associated with it.
			metricDescriptor := getOTLPMetricDescriptor(metricKey)
			int64Val, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				z.logger.Debug(
					fmt.Sprintf("non-integer value from %s", mntrCommand),
					zap.String("value", metricValue),
				)
				continue
			}
			stats = append(stats, stat{metric: metricDescriptor, val: int64Val})
		}
	}

	return stats, attributes
}

func closeConnection(conn net.Conn) error {
	return conn.Close()
}

func setConnectionDeadline(conn net.Conn, deadline time.Time) error {
	return conn.SetDeadline(deadline)
}

func sendCmd(conn net.Conn, cmd string) (*bufio.Scanner, error) {
	_, err := fmt.Fprintf(conn, "%s\n", cmd)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	scanner := bufio.NewScanner(reader)
	return scanner, nil
}
