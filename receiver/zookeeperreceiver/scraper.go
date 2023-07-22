// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver/internal/metadata"
)

var zookeeperFormatRE = regexp.MustCompile(`(^zk_\w+)\s+([\w\.\-]+)`)

const (
	mntrCommand = "mntr"
	ruokCommand = "ruok"
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
	return metadata.Type
}

func newZookeeperMetricsScraper(settings receiver.CreateSettings, config *Config) (*zookeeperMetricsScraper, error) {
	_, _, err := net.SplitHostPort(config.TCPAddr.Endpoint)
	if err != nil {
		return nil, err
	}

	if config.Timeout <= 0 {
		return nil, errors.New("timeout must be a positive duration")
	}

	z := &zookeeperMetricsScraper{
		logger:                settings.Logger,
		config:                config,
		mb:                    metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		closeConnection:       closeConnection,
		setConnectionDeadline: setConnectionDeadline,
		sendCmd:               sendCmd,
	}

	return z, nil
}

func (z *zookeeperMetricsScraper) shutdown(_ context.Context) error {
	if z.cancel != nil {
		z.cancel()
		z.cancel = nil
	}
	return nil
}

func (z *zookeeperMetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var ctxWithTimeout context.Context
	ctxWithTimeout, z.cancel = context.WithTimeout(ctx, z.config.Timeout)

	responseMntr, err := z.runCommand(ctxWithTimeout, "mntr")
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	responseRuok, err := z.runCommand(ctxWithTimeout, "ruok")
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	resourceOpts := make([]metadata.ResourceMetricsOption, 0, 2)

	resourceOpts = z.processMntr(responseMntr, resourceOpts)
	z.processRuok(responseRuok)

	return z.mb.Emit(resourceOpts...), nil
}

func (z *zookeeperMetricsScraper) runCommand(ctx context.Context, command string) ([]string, error) {
	conn, err := z.config.Dial()

	if err != nil {
		z.logger.Error("failed to establish connection",
			zap.String("endpoint", z.config.Endpoint),
			zap.Error(err),
		)
		return nil, err
	}
	defer func() {
		if closeErr := z.closeConnection(conn); closeErr != nil {
			z.logger.Warn("failed to shutdown connection", zap.Error(closeErr))
		}
	}()

	deadline, ok := ctx.Deadline()
	if ok {
		if err = z.setConnectionDeadline(conn, deadline); err != nil {
			z.logger.Warn("failed to set deadline on connection", zap.Error(err))
		}
	}

	scanner, err := z.sendCmd(conn, command)
	if err != nil {
		z.logger.Error("failed to send command",
			zap.Error(err),
			zap.String("command", command),
		)
		return nil, err
	}

	var response []string
	for scanner.Scan() {
		response = append(response, scanner.Text())
	}
	return response, nil
}

func (z *zookeeperMetricsScraper) processMntr(response []string, resourceOpts []metadata.ResourceMetricsOption) []metadata.ResourceMetricsOption {
	creator := newMetricCreator(z.mb)
	now := pcommon.NewTimestampFromTime(time.Now())
	for _, line := range response {
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
			resourceOpts = append(resourceOpts, metadata.WithZkVersion(metricValue))
			continue
		case serverStateKey:
			resourceOpts = append(resourceOpts, metadata.WithServerState(metricValue))
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
	return resourceOpts
}

func (z *zookeeperMetricsScraper) processRuok(response []string) {
	creator := newMetricCreator(z.mb)
	now := pcommon.NewTimestampFromTime(time.Now())

	metricKey := "ruok"
	metricValue := int64(0)

	if len(response) > 0 {
		if response[0] == "imok" {
			metricValue = int64(1)
		} else {
			z.logger.Error("invalid response from ruok",
				zap.String("command", ruokCommand),
			)
			return
		}
	}

	recordDataPoints := creator.recordDataPointsFunc(metricKey)
	recordDataPoints(now, metricValue)
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
