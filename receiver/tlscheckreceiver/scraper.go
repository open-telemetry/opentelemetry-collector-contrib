// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

var (
	ErrMissingTargets = errors.New(`No targets specified`)
)

type scraper struct {
	cfg                *Config
	settings           component.TelemetrySettings
	mb                 *metadata.MetricsBuilder
	getConnectionState func(host string) (tls.ConnectionState, error)
}

func getConnectionState(host string) (tls.ConnectionState, error) {
	conn, err := tls.Dial("tcp", host, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return tls.ConnectionState{}, err
	}
	defer conn.Close()
	return conn.ConnectionState(), nil
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	if s.cfg == nil || len(s.cfg.Targets) == 0 {
		return pmetric.NewMetrics(), ErrMissingTargets
	}

	var wg sync.WaitGroup
	wg.Add(len(s.cfg.Targets))
	var mux sync.Mutex
	for _, target := range s.cfg.Targets {
		go func(host string) {
			defer wg.Done()

			now := pcommon.NewTimestampFromTime(time.Now())
			mux.Lock()
			state, err := s.getConnectionState(target.Host)
			if err != nil {
				s.settings.Logger.Error("TCP connection error encountered", zap.String("host", target.Host), zap.Error(err))
			}

			s.settings.Logger.Error("Peer Certificates", zap.Int("certificates_count", len(state.PeerCertificates)))
			if len(state.PeerCertificates) == 0 {
				s.settings.Logger.Error("No TLS certificates found. Verify the host serves TLS certificates.", zap.String("host", target.Host))
			}

			cert := state.PeerCertificates[0]
			issuer := cert.Issuer.String()
			commonName := cert.Subject.CommonName
			currentTime := time.Now()
			timeLeft := cert.NotAfter.Sub(currentTime).Seconds()
			timeLeftInt := int64(timeLeft)
			s.mb.RecordTlscheckTimeLeftDataPoint(now, timeLeftInt, issuer, commonName, host)
			mux.Unlock()

		}(target.Host)
	}

	wg.Wait()
	return s.mb.Emit(), nil
}

func newScraper(cfg *Config, settings receiver.Settings, getConnectionState func(host string) (tls.ConnectionState, error)) *scraper {
	return &scraper{
		cfg:                cfg,
		settings:           settings.TelemetrySettings,
		mb:                 metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
		getConnectionState: getConnectionState,
	}
}
