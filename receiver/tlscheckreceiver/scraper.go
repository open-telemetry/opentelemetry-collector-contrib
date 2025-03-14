// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

var errMissingTargets = errors.New(`No targets specified`)

type scraper struct {
	cfg                *Config
	settings           receiver.Settings
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

func (s *scraper) scrapeEndpoint(endpoint string, metrics *pmetric.Metrics, wg *sync.WaitGroup, mux *sync.Mutex) {
	defer wg.Done()

	state, err := s.getConnectionState(endpoint)
	if err != nil {
		s.settings.Logger.Error("TCP connection error encountered", zap.String("endpoint", endpoint), zap.Error(err))
		return
	}

	s.settings.Logger.Info("Peer Certificates", zap.Int("certificates_count", len(state.PeerCertificates)))
	if len(state.PeerCertificates) == 0 {
		s.settings.Logger.Error("No TLS certificates found. Verify the endpoint serves TLS certificates.", zap.String("endpoint", endpoint))
		return
	}

	cert := state.PeerCertificates[0]
	issuer := cert.Issuer.String()
	commonName := cert.Subject.CommonName
	currentTime := time.Now()
	timeLeft := cert.NotAfter.Sub(currentTime).Seconds()
	timeLeftInt := int64(timeLeft)
	now := pcommon.NewTimestampFromTime(time.Now())

	mux.Lock()
	defer mux.Unlock()

	mb := metadata.NewMetricsBuilder(s.cfg.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.NewTimestampFromTime(time.Now())))
	rb := mb.NewResourceBuilder()
	rb.SetTlscheckEndpoint(endpoint)
	mb.RecordTlscheckTimeLeftDataPoint(now, timeLeftInt, issuer, commonName)
	resourceMetrics := mb.Emit(metadata.WithResource(rb.Emit()))
	resourceMetrics.ResourceMetrics().At(0).MoveTo(metrics.ResourceMetrics().AppendEmpty())
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	if s.cfg == nil || len(s.cfg.Targets) == 0 {
		return pmetric.NewMetrics(), errMissingTargets
	}

	var wg sync.WaitGroup
	wg.Add(len(s.cfg.Targets))
	var mux sync.Mutex

	metrics := pmetric.NewMetrics()

	for _, target := range s.cfg.Targets {
		go s.scrapeEndpoint(target.Endpoint, &metrics, &wg, &mux)
	}

	wg.Wait()
	return metrics, nil
}

func newScraper(cfg *Config, settings receiver.Settings, getConnectionState func(endpoint string) (tls.ConnectionState, error)) *scraper {
	return &scraper{
		cfg:                cfg,
		settings:           settings,
		getConnectionState: getConnectionState,
	}
}
