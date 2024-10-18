// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/configtcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

var errClientNotInit = errors.New("client not initialized")

type tcpCheckScraper struct {
	*configtcp.Client
	*Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

// start the scraper by creating a new TCP Client on the scraper
func (s *tcpCheckScraper) start(_ context.Context, host component.Host) error {
	var err error
	s.Client, err = s.Config.ToClient(host, s.settings)
	return err
}

func (s *tcpCheckScraper) scrapeTCP(now pcommon.Timestamp) error {
	var success int64

	start := time.Now()
	err := s.Client.Dial()
	if err == nil {
		success = 1
	}
	s.mb.RecordTcpcheckDurationDataPoint(now, time.Since(start).Nanoseconds(), s.Config.TCPClientSettings.Endpoint)
	s.mb.RecordTcpcheckStatusDataPoint(now, success, s.Config.TCPClientSettings.Endpoint)
	return err
}

// timeout chooses the shorter between a given deadline and timeout
func timeout(deadline time.Time, timeout time.Duration) time.Duration {
	timeToDeadline := time.Until(deadline)
	if timeToDeadline < timeout {
		return timeToDeadline
	}
	return timeout
}

// scrape connects to the endpoint and produces metrics based on the response.
func (s *tcpCheckScraper) scrape(ctx context.Context) (_ pmetric.Metrics, err error) {
	var (
		to time.Duration
	)
	// check cancellation
	select {
	case <-ctx.Done():
		return pmetric.NewMetrics(), ctx.Err()
	default:
	}

	cleanup := func() {
		s.Client.Close()
	}

	// if the context carries a shorter deadline then timeout that quickly
	deadline, ok := ctx.Deadline()
	if ok {
		to = timeout(deadline, s.Client.TCPAddrConfig.DialerConfig.Timeout)
		s.Client.TCPAddrConfig.DialerConfig.Timeout = to
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	now := pcommon.NewTimestampFromTime(time.Now())
	if s.Client == nil {
		return pmetric.NewMetrics(), errClientNotInit
	}

	if err = s.scrapeTCP(now); err != nil {
		s.mb.RecordTcpcheckErrorDataPoint(now, int64(1), s.Endpoint, err.Error())
	} else {
		go func() {
			<-ctx.Done()
			cleanup()
		}()
	}

	return s.mb.Emit(), nil
}

func newScraper(conf *Config, settings receiver.Settings) *tcpCheckScraper {
	return &tcpCheckScraper{
		Config:   conf,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
	}
}
