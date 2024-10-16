// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"context"
	"crypto/tls"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

var (
	errMissingHost = errors.New(`No targets specified`)
)

type scraper struct {
	cfg    *Config
	logger *zap.Logger
	mb     *metadata.MetricsBuilder
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
  if len(t.cfg.Targets) == 0 {
      return pmetric.NewMetrics(), errMissingHost
  }

  var wg sync.WaitGroup
  wg.Add(len(s.cfg.Targets))
  var mux sync.Mutex
  for _, host := range s.cfg.Targets {
      go func(host string) {
          defer wg.Done()

          now := pcommon.NewTimestampFromTime(time.Now())
          start := time.Now()

          conn, err := tls.Dial("tcp", host, &tls.Config{
						InsecureSkipVerify: true,
					})
          if err != nil {
              mux.Lock()
              s.mb.RecordTlscheckErrorDataPoint(now, int64(1), host, err.Error())
              mux.Unlock()
              return
          }
          defer conn.Close()

          cert := conn.ConnectionState().PeerCertificates[0]
          issuer := cert.Issuer.String()
          commonName := cert.Subject.CommonName

          mux.Lock()
          defer mux.Unlock()

          currentTime := time.Now()
          timeLeft := cert.NotAfter.Sub(currentTime).Seconds()

          s.mb.RecordTlscheckTimeLeftDataPoint(now, timeLeft, host, issuer, commonName)
          
      }(endpoint)
  }

  wg.Wait()
  return t.mb.Emit(), nil
}

func newScraper(cfg *Config, settings receiver.Settings) *scraper {
	return &scraper{
		logger: settings.TelemetrySettings.Logger,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}
