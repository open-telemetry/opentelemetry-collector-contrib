// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

type scraper struct {
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
	// dial sth
	getConnectionState func(endpoint string) (tls.ConnectionState, error)
}

func getConnectionState(endpoint string) (tls.ConnectionState, error) {
	conn, err := tls.Dial("tcp", endpoint, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return tls.ConnectionState{}, err
	}
	defer conn.Close()
	return conn.ConnectionState(), nil
}

func (s *scraper) scrapeEndpoint(endpoint string, wg *sync.WaitGroup, mux *sync.Mutex) {
	defer wg.Done()
	const pointVal int64 = 1 // Use a constant for clarity and immutability
	start := time.Now()

	// Attempt to get the connection state
	_, err := s.getConnectionState(endpoint)
	now := pcommon.NewTimestampFromTime(time.Now())

	if err != nil {
		// Record error data point and log the error
		mux.Lock()
		s.mb.RecordTcpcheckErrorDataPoint(now, pointVal, endpoint, err.Error())
		s.settings.Logger.Error("TCP connection error encountered", zap.String("endpoint", endpoint), zap.Error(err))
		defer mux.Unlock()
		return
	}

	// Record success metrics
	duration := time.Since(start).Milliseconds()

	mux.Lock()
	s.mb.RecordTcpcheckDurationDataPoint(now, duration, endpoint)
	s.mb.RecordTcpcheckStatusDataPoint(now, pointVal, endpoint)
	defer mux.Unlock()
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	if s.cfg == nil || len(s.cfg.Targets) == 0 {
		return pmetric.NewMetrics(), errMissingTargets
	}

	var wg sync.WaitGroup
	wg.Add(len(s.cfg.Targets))
	var mux sync.Mutex

	for _, target := range s.cfg.Targets {
		go s.scrapeEndpoint(target.Endpoint, &wg, &mux)
	}

	wg.Wait()
	return s.mb.Emit(), nil
}

func newScraper(cfg *Config, settings receiver.Settings, getConnectionState func(endpoint string) (tls.ConnectionState, error)) *scraper {
	return &scraper{
		cfg:                cfg,
		settings:           settings.TelemetrySettings,
		mb:                 metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
		getConnectionState: getConnectionState,
	}
}
