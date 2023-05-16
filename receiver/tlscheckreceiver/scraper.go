// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"
import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/configtls"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

var (
	errClientNotInit = errors.New("client not initialized")
)

type tlsCheckScraper struct {
	*configtls.Client
	*Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

func newScraper(conf *Config, settings receiver.CreateSettings) *tlsCheckScraper {
	return &tlsCheckScraper{
		Config:   conf,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
	}
}

// start starts the scraper by creating a new HTTP Client on the scraper
func (t *tlsCheckScraper) start(ctx context.Context, host component.Host) (err error) {
	t.Client, err = t.Config.ToClient(host, t.settings)
	return
}

func (t *tlsCheckScraper) scrapeLocalTLSCheck(now pcommon.Timestamp, cert *x509.Certificate) (err error) {
	secondsUntilExpiration := int64(cert.NotAfter.Sub(time.Now()).Seconds())
	daysUntilExpiration := int64(cert.NotAfter.Sub(time.Now()).Hours() / 24)
	secondsIssuedFor := int64(cert.NotAfter.Sub(cert.NotBefore).Seconds())
	daysIssuedFor := int64(cert.NotAfter.Sub(cert.NotBefore).Hours() / 24)

	t.mb.RecordTlscheckDaysLeftDataPoint(now, daysUntilExpiration)
	t.mb.RecordTlscheckIssuedDaysDataPoint(now, secondsUntilExpiration)
	t.mb.RecordTlscheckIssuedSecondsDataPoint(now, daysIssuedFor)
	t.mb.RecordTlscheckSecondsLeftDataPoint(now, secondsIssuedFor)
	return err
}

func (t *tlsCheckScraper) scrapeRemoteTLSCheck(now pcommon.Timestamp) (err error) {
	err = t.Client.DialWithDialer(t.Config.Endpoint)
	if err != nil {
		return err
	}
	certificates := t.Client.ConnectionState().PeerCertificates
	for _, cert := range certificates {
		secondsUntilExpiration := int64(cert.NotAfter.Sub(time.Now()).Seconds())
		daysUntilExpiration := int64(cert.NotAfter.Sub(time.Now()).Hours() / 24)
		secondsIssuedFor := int64(cert.NotAfter.Sub(cert.NotBefore).Seconds())
		daysIssuedFor := int64(cert.NotAfter.Sub(cert.NotBefore).Hours() / 24)

		t.mb.RecordTlscheckDaysLeftDataPoint(now, daysUntilExpiration)
		t.mb.RecordTlscheckIssuedDaysDataPoint(now, secondsUntilExpiration)
		t.mb.RecordTlscheckIssuedSecondsDataPoint(now, daysIssuedFor)
		t.mb.RecordTlscheckSecondsLeftDataPoint(now, secondsIssuedFor)
	}

	return err
}

// scrape connects to the endpoint and produces metrics based on the response
func (t *tlsCheckScraper) scrape(ctx context.Context) (_ pmetric.Metrics, err error) {
	if t.Client == nil {
		return pmetric.NewMetrics(), errClientNotInit
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	if t.Config.LocalCertPath != "" {
		cert, err := t.getLocalCert()
		if err != nil {
			return pmetric.NewMetrics(), err
		}

		if err = t.scrapeLocalTLSCheck(now, cert); err != nil {
			t.mb.RecordTlscheckErrorDataPoint(now, int64(1), err.Error())
		}
	} else {
		if err = t.scrapeRemoteTLSCheck(now); err != nil {
			t.mb.RecordTlscheckErrorDataPoint(now, int64(1), err.Error())
		}
	}

	return t.mb.Emit(), nil
}

func (t *tlsCheckScraper) getLocalCert() (*x509.Certificate, error) {
	certPEM, err := os.ReadFile(t.Config.LocalCertPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %v", err)
	}
	return cert, nil
}
