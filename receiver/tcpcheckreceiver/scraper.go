// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

type scraper struct {
	cfg                *Config
	settings           component.TelemetrySettings
	mb                 *metadata.MetricsBuilder
	getConnectionState func(tcpConfig *confignet.TCPAddrConfig) (tcpConnectionState, error)
	errorCounts        map[string]int64
}

type tcpConnectionState struct {
	LocalAddr  string // Local address of the connection
	RemoteAddr string // Remote address of the connection
	Network    string // Network type (e.g., "tcp")
}

func getConnectionState(tcpConfig *confignet.TCPAddrConfig) (tcpConnectionState, error) {
	conn, err := tcpConfig.Dial(context.Background())
	if err != nil {
		return tcpConnectionState{}, err
	}
	defer conn.Close()
	state := tcpConnectionState{
		LocalAddr:  conn.LocalAddr().String(),
		RemoteAddr: conn.RemoteAddr().String(),
		Network:    conn.LocalAddr().Network(),
	}
	return state, nil
}

func (s *scraper) errorListener(ctx context.Context, eQueue <-chan error, eOut chan<- *scrapererror.ScrapeErrors) {
	errs := &scrapererror.ScrapeErrors{}

	for {
		select {
		case <-ctx.Done():
			eOut <- errs
			return
		case err, ok := <-eQueue:
			if !ok {
				eOut <- errs
				return
			}
			errs.AddPartial(1, err)
		}
	}
}

func (s *scraper) scrapeEndpoint(ctx context.Context, tcpConfig *confignet.TCPAddrConfig, wg *sync.WaitGroup, mux *sync.Mutex, errChan chan<- error) {
	defer wg.Done()
	const pointValue int64 = 1
	const failPointValue int64 = 0

	start := time.Now()
	_, err := s.getConnectionState(tcpConfig)
	now := pcommon.NewTimestampFromTime(time.Now())

	duration := time.Since(start).Milliseconds()

	mux.Lock()
	defer mux.Unlock()

	if err != nil {
		// Convert error to appropriate error code
		var errorCode metadata.AttributeErrorCode
		errMsg := strings.ToLower(err.Error())

		switch {
		case strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "connection reset by peer") ||
			strings.Contains(errMsg, "no route to host") ||
			strings.Contains(errMsg, "host is down") ||
			strings.Contains(errMsg, "no connection could be made because the target machine actively refused it"):
			errorCode = metadata.AttributeErrorCodeConnectionRefused
		case strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "i/o timeout") ||
			strings.Contains(errMsg, "operation timed out"):
			errorCode = metadata.AttributeErrorCodeConnectionTimeout
		case strings.Contains(errMsg, "invalid endpoint") ||
			strings.Contains(errMsg, "missing port in address") ||
			strings.Contains(errMsg, "invalid port") ||
			strings.Contains(errMsg, "invalid host") ||
			strings.Contains(errMsg, "invalid address") ||
			strings.Contains(errMsg, "invalid hostname") ||
			strings.Contains(errMsg, "invalid ip") ||
			strings.Contains(errMsg, "invalid tcp address") ||
			strings.Contains(errMsg, "invalid:host") ||
			strings.Contains(errMsg, "missing port") ||
			strings.Contains(errMsg, "address invalid") ||
			strings.Contains(errMsg, "unknown port") ||
			strings.Contains(errMsg, "unknown host"):
			errorCode = metadata.AttributeErrorCodeInvalidEndpoint
		case strings.Contains(errMsg, "network is unreachable") ||
			strings.Contains(errMsg, "network unreachable") ||
			strings.Contains(errMsg, "no such network interface") ||
			strings.Contains(errMsg, "network down"):
			errorCode = metadata.AttributeErrorCodeNetworkUnreachable
		default:
			errorCode = metadata.AttributeErrorCodeUnknownError
		}

		// Increment error count for this endpoint
		s.errorCounts[tcpConfig.Endpoint]++

		// Record error metrics with the current error count
		s.mb.RecordTcpcheckErrorDataPoint(now, s.errorCounts[tcpConfig.Endpoint], tcpConfig.Endpoint, errorCode)

		s.mb.RecordTcpcheckStatusDataPoint(now, failPointValue, tcpConfig.Endpoint)
		// Send error to channel
		select {
		case <-ctx.Done():
			return
		case errChan <- err:
		}
		return
	}

	// Record success metrics
	s.mb.RecordTcpcheckDurationDataPoint(now, duration, tcpConfig.Endpoint)
	s.mb.RecordTcpcheckStatusDataPoint(now, pointValue, tcpConfig.Endpoint)
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if s.cfg == nil || len(s.cfg.Targets) == 0 {
		return pmetric.NewMetrics(), errMissingTargets
	}
	// Initialize error counts if not already done
	if s.errorCounts == nil {
		s.errorCounts = make(map[string]int64)
	}

	var wg sync.WaitGroup
	var errs *scrapererror.ScrapeErrors
	var mux sync.Mutex

	errOut := make(chan *scrapererror.ScrapeErrors)
	errChan := make(chan error, len(s.cfg.Targets))

	// Start error listener
	go func() {
		s.errorListener(ctx, errChan, errOut)
	}()

	wg.Add(len(s.cfg.Targets))
	for _, tcpConfig := range s.cfg.Targets {
		go s.scrapeEndpoint(ctx, tcpConfig, &wg, &mux, errChan)
	}

	wg.Wait()
	close(errChan)
	errs = <-errOut

	return s.mb.Emit(), errs.Combine()
}

func newScraper(cfg *Config, settings receiver.Settings) *scraper {
	return &scraper{
		cfg:                cfg,
		settings:           settings.TelemetrySettings,
		mb:                 metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
		getConnectionState: getConnectionState,
	}
}
