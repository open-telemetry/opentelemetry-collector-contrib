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

package zookeeperreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
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
	mb     *metadata.MetricsBuilder

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
		mb:                    metadata.NewMetricsBuilder(config.Metrics),
		closeConnection:       closeConnection,
		setConnectionDeadline: setConnectionDeadline,
		sendCmd:               sendCmd,
	}, nil
}

func (z *zookeeperMetricsScraper) shutdown(_ context.Context) error {
	if z.cancel != nil {
		z.cancel()
		z.cancel = nil
	}
	return nil
}

func (z *zookeeperMetricsScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	var ctxWithTimeout context.Context
	ctxWithTimeout, z.cancel = context.WithTimeout(ctx, z.config.Timeout)

	conn, err := z.config.Dial()
	if err != nil {
		z.logger.Error("failed to establish connection",
			zap.String("endpoint", z.config.Endpoint),
			zap.Error(err),
		)
		return pdata.NewMetrics(), err
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

func (z *zookeeperMetricsScraper) getResourceMetrics(conn net.Conn) (pdata.Metrics, error) {
	scanner, err := z.sendCmd(conn, mntrCommand)
	if err != nil {
		z.logger.Error("failed to send command",
			zap.Error(err),
			zap.String("command", mntrCommand),
		)
		return pdata.NewMetrics(), err
	}

	md := z.mb.NewMetricData()
	creator := newMetricCreator(z.mb)
	now := pdata.NewTimestampFromTime(time.Now())
	rm := md.ResourceMetrics().At(0)
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
			rm.Resource().Attributes().UpsertString(metadata.Attributes.ZkVersion, metricValue)
			continue
		case serverStateKey:
			rm.Resource().Attributes().UpsertString(metadata.Attributes.ServerState, metricValue)
			continue
		default:
			// Skip metric if there is no descriptor associated with it.
			recordDataPoints := creator.recordDataPointsFunc(metricKey)
			if recordDataPoints == nil {
				// Unexported metric, just move to the next line.
				continue
			}
			int64Val, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				z.logger.Debug(
					fmt.Sprintf("non-integer value from %s", mntrCommand),
					zap.String("value", metricValue),
				)
				continue
			}
			recordDataPoints(now, int64Val)
		}
	}

	// Generate computed metrics
	creator.generateComputedMetrics(z.logger, now)

	z.mb.Emit(rm.InstrumentationLibraryMetrics().At(0).Metrics())
	return md, nil
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
