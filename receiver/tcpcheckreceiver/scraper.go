// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

type scraper struct {
	cfg                *Config
	settings           component.TelemetrySettings
	mb                 *metadata.MetricsBuilder
	getConnectionState func(tcpConfig *confignet.TCPAddrConfig) (TCPConnectionState, error)
}

type TCPConnectionState struct {
	LocalAddr  string // Local address of the connection
	RemoteAddr string // Remote address of the connection
	Network    string // Network type (e.g., "tcp")
}

func getConnectionState(tcpConfig *confignet.TCPAddrConfig) (TCPConnectionState, error) {
	conn, err := tcpConfig.Dial(context.Background())
	if err != nil {
		return TCPConnectionState{}, err
	}
	defer conn.Close()
	state := TCPConnectionState{
		LocalAddr:  conn.LocalAddr().String(),
		RemoteAddr: conn.RemoteAddr().String(),
		Network:    conn.LocalAddr().Network(),
	}
	return state, nil
}

func (s *scraper) scrapeEndpoint(tcpConfig *confignet.TCPAddrConfig, wg *sync.WaitGroup, mux *sync.Mutex) {
	defer wg.Done()
	const pointValue int64 = 1

	start := time.Now()
	_, err := s.getConnectionState(tcpConfig)
	now := pcommon.NewTimestampFromTime(time.Now())

	duration := time.Since(start).Milliseconds()

	mux.Lock()
	defer mux.Unlock()

	if err != nil {
		// Record error data point and log the error
		s.mb.RecordTcpcheckErrorDataPoint(now, pointValue, tcpConfig.Endpoint, err.Error())
		s.settings.Logger.Error("TCP connection error encountered", zap.String("endpoint", tcpConfig.Endpoint), zap.Error(err))
		return
	}

	// Record success data points
	s.mb.RecordTcpcheckDurationDataPoint(now, duration, tcpConfig.Endpoint)
	s.mb.RecordTcpcheckStatusDataPoint(now, pointValue, tcpConfig.Endpoint)
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	if s.cfg == nil || len(s.cfg.Targets) == 0 {
		return pmetric.NewMetrics(), errMissingTargets
	}

	var wg sync.WaitGroup
	wg.Add(len(s.cfg.Targets))
	var mux sync.Mutex

	for _, tcpConfig := range s.cfg.Targets {
		// endpoint and dialer
		go s.scrapeEndpoint(tcpConfig, &wg, &mux)
	}

	wg.Wait()
	return s.mb.Emit(), nil
}

func newScraper(cfg *Config, settings receiver.Settings) *scraper {
	return &scraper{
		cfg:                cfg,
		settings:           settings.TelemetrySettings,
		mb:                 metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
		getConnectionState: getConnectionState,
	}
}
