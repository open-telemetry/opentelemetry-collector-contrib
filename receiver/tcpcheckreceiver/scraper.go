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

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

type tcpcheckScraper struct {
	net.Conn
	*Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

// start starts the scraper by creating a new net Client on the scraper
func (s *tcpcheckScraper) start(_ context.Context, host component.Host) error {
	return nil
}

// timeout chooses the shorter between between a given deadline and timeout
func timeout(deadline time.Time, timeout time.Duration) time.Duration {
	timeToDeadline := time.Until(deadline)
	if timeToDeadline < timeout {
		return timeToDeadline
	}
	return timeout
}

// scrape connects to the endpoint and produces metrics based on the response.
func (s *tcpcheckScraper) scrape(ctx context.Context) (_ pmetric.Metrics, err error) {
	var (
		success int64
		to      time.Duration
	)

	deadline, ok := ctx.Deadline()
	if ok {
		to = timeout(deadline, s.Timeout)
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	start := time.Now()

	select {
	case <-ctx.Done():
		s.mb.RecordTcpcheckErrorDataPoint(now, int64(1), s.Endpoint, s.Transport, ctx.Err().Error())
	default:
		s.Conn, err = net.DialTimeout(s.Transport, s.Endpoint, to)
		if err != nil {
			s.mb.RecordTcpcheckErrorDataPoint(now, int64(1), s.Endpoint, s.Transport, err.Error())
		} else {
			success = 1
			defer func() {
				err = s.Conn.Close()
			}()
		}
		s.mb.RecordTcpcheckDurationDataPoint(now, time.Since(start).Milliseconds(), s.Endpoint, s.Transport)
		s.mb.RecordTcpcheckStatusDataPoint(now, success, s.Endpoint, s.Transport)
	}

	return s.mb.Emit(), nil
}

func newScraper(conf *Config, settings receiver.CreateSettings) *tcpcheckScraper {
	return &tcpcheckScraper{
		Config:   conf,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
	}
}
