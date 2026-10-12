// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
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
	getConnectionState func(ctx context.Context, target *compiledTarget) (tcpConnectionState, error)
	errorCounts        map[string]int64
	targets            []*compiledTarget
}

// compiledTarget is a TargetConfig whose query_response steps have been
// compiled once, so that no regex compilation or base64 decoding happens on
// every check.
type compiledTarget struct {
	*TargetConfig
	steps []compiledStep
}

func compileTargets(cfg *Config) ([]*compiledTarget, error) {
	targets := make([]*compiledTarget, 0, len(cfg.Targets))
	for _, target := range cfg.Targets {
		ct := &compiledTarget{
			TargetConfig: target,
			steps:        make([]compiledStep, 0, len(target.QueryResponse)),
		}
		for i := range target.QueryResponse {
			step, err := target.QueryResponse[i].compile()
			if err != nil {
				return nil, fmt.Errorf("target %s: query_response step %d: %w", target.Endpoint, i, err)
			}
			ct.steps = append(ct.steps, step)
		}
		targets = append(targets, ct)
	}
	return targets, nil
}

type tcpConnectionState struct {
	LocalAddr  string // Local address of the connection
	RemoteAddr string // Remote address of the connection
	Network    string // Network type (e.g., "tcp")
}

func getConnectionState(ctx context.Context, target *compiledTarget) (tcpConnectionState, error) {
	start := time.Now()
	conn, err := target.Dial(ctx)
	if err != nil {
		return tcpConnectionState{}, err
	}
	defer conn.Close()
	state := tcpConnectionState{
		LocalAddr:  conn.LocalAddr().String(),
		RemoteAddr: conn.RemoteAddr().String(),
		Network:    conn.LocalAddr().Network(),
	}
	if len(target.steps) == 0 {
		return state, nil
	}
	// The dialer timeout bounds the whole check, including the exchange.
	deadline := exchangeDeadline(ctx, start, target.DialerConfig.Timeout)
	if err := runQueryResponse(ctx, conn, target.steps, target.MaxResponseBytes, deadline); err != nil {
		return state, err
	}
	return state, nil
}

// errorCodeFor maps a check failure to the error.code attribute value.
func errorCodeFor(err error) metadata.AttributeErrorCode {
	if _, ok := errors.AsType[*queryResponseError](err); ok {
		return metadata.AttributeErrorCodeQueryResponseFailed
	}

	errMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset by peer") ||
		strings.Contains(errMsg, "no route to host") ||
		strings.Contains(errMsg, "host is down") ||
		strings.Contains(errMsg, "no connection could be made because the target machine actively refused it"):
		return metadata.AttributeErrorCodeConnectionRefused
	case strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "i/o timeout") ||
		strings.Contains(errMsg, "operation timed out"):
		return metadata.AttributeErrorCodeConnectionTimeout
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
		return metadata.AttributeErrorCodeInvalidEndpoint
	case strings.Contains(errMsg, "network is unreachable") ||
		strings.Contains(errMsg, "network unreachable") ||
		strings.Contains(errMsg, "no such network interface") ||
		strings.Contains(errMsg, "network down"):
		return metadata.AttributeErrorCodeNetworkUnreachable
	default:
		return metadata.AttributeErrorCodeUnknownError
	}
}

func (*scraper) errorListener(ctx context.Context, eQueue <-chan error, eOut chan<- *scrapererror.ScrapeErrors) {
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

func (s *scraper) scrapeEndpoint(ctx context.Context, target *compiledTarget, wg *sync.WaitGroup, mux *sync.Mutex, errChan chan<- error) {
	defer wg.Done()
	const pointValue int64 = 1
	const failPointValue int64 = 0

	start := time.Now()
	_, err := s.getConnectionState(ctx, target)
	now := pcommon.NewTimestampFromTime(time.Now())

	duration := time.Since(start).Milliseconds()

	mux.Lock()
	defer mux.Unlock()

	if err != nil {
		errorCode := errorCodeFor(err)

		// Increment error count for this endpoint
		s.errorCounts[target.Endpoint]++

		// Record error metrics with the current error count
		s.mb.RecordTcpcheckErrorDataPoint(now, s.errorCounts[target.Endpoint], target.Endpoint, errorCode)

		s.mb.RecordTcpcheckStatusDataPoint(now, failPointValue, target.Endpoint)
		// Send error to channel
		select {
		case <-ctx.Done():
			return
		case errChan <- err:
		}
		return
	}

	// Record success metrics
	s.mb.RecordTcpcheckDurationDataPoint(now, duration, target.Endpoint)
	s.mb.RecordTcpcheckStatusDataPoint(now, pointValue, target.Endpoint)
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if len(s.targets) == 0 {
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
	errChan := make(chan error, len(s.targets))

	// Start error listener
	go func() {
		s.errorListener(ctx, errChan, errOut)
	}()

	wg.Add(len(s.targets))
	for _, target := range s.targets {
		go s.scrapeEndpoint(ctx, target, &wg, &mux, errChan)
	}

	wg.Wait()
	close(errChan)
	errs = <-errOut

	return s.mb.Emit(), errs.Combine()
}

func newScraper(cfg *Config, settings receiver.Settings) (*scraper, error) {
	targets, err := compileTargets(cfg)
	if err != nil {
		return nil, err
	}
	return &scraper{
		cfg:                cfg,
		settings:           settings.TelemetrySettings,
		mb:                 metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), settings),
		getConnectionState: getConnectionState,
		targets:            targets,
	}, nil
}
