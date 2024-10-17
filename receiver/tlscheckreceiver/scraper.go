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

var (
	ErrMissingTargets = errors.New(`No targets specified`)
)

type scraper struct {
	cfg    *Config
	logger *zap.Logger
	mb     *metadata.MetricsBuilder
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

          conn, err := tls.Dial("tcp", target.Host, &tls.Config{
						InsecureSkipVerify: true,
					})
          if err != nil {
              s.logger.Error("TCP connection error encountered", zap.String("host", target.Host), zap.Error(err))
              return
          }
          defer conn.Close()

          // Ensure conn is not nil before accessing its methods
          if conn == nil {
              s.logger.Error("Failed to establish a connection", zap.String("host", target.Host))
              return
          }

          state := conn.ConnectionState()
          if len(state.PeerCertificates) == 0 {
            s.logger.Error("No TLS certificates found. Verify the host serves TLS certificates.", zap.String("host", target.Host))
            return
          }

          cert := conn.ConnectionState().PeerCertificates[0]
          issuer := cert.Issuer.String()
          commonName := cert.Subject.CommonName

          mux.Lock()
          defer mux.Unlock()

          currentTime := time.Now()
          timeLeft := cert.NotAfter.Sub(currentTime).Seconds()
          timeLeftInt := int64(timeLeft)

          s.mb.RecordTlscheckTimeLeftDataPoint(now, timeLeftInt, issuer, commonName)
          
      }(target.Host)
  }

  wg.Wait()
  return s.mb.Emit(), nil
}

func newScraper(cfg *Config, settings receiver.Settings) *scraper {
	return &scraper{
		cfg:    cfg,
		logger: settings.TelemetrySettings.Logger,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}
