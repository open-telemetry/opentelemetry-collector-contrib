// Copyright  The OpenTelemetry Authors
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

package haproxyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver"

import (
	"bytes"
	"context"
	"encoding/csv"
	"net"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver/internal/metadata"
)

var (
	showStatsCommand = []byte("show stats\n")
)

type scraper struct {
	endpoint       string
	logger         *zap.Logger
	metricsBuilder *metadata.MetricsBuilder
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "unix", s.endpoint)
	if err != nil {
		return pmetric.NewMetrics(), err
	}
	defer c.Close()
	records, err := s.readStats(c)
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	for _, record := range records {
		err = s.metricsBuilder.RecordHaproxySessionsCountDataPoint(now, record["scur"], record["pxname"], record["svname"])
		if err != nil {
			return pmetric.NewMetrics(), err
		}
	}

	return s.metricsBuilder.Emit(metadata.WithHaproxyAddr(s.endpoint)), nil
}

func (s *scraper) readStats(c net.Conn) ([]map[string]string, error) {
	_, err := c.Write(showStatsCommand)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	_, err = c.Read(buf)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(buf))
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}
	// The CSV output starts with `# `, removing it to be able to read headers.
	headers[0] = strings.TrimPrefix(headers[0], "# ")
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	results := make([]map[string]string, len(rows))
	for i, record := range rows {
		result := make(map[string]string)
		results[i] = result
		for j, header := range headers {
			result[header] = record[j]
		}
	}

	return results, err
}

func newScraper(metricsBuilder *metadata.MetricsBuilder, cfg *Config, logger *zap.Logger) *scraper {
	return &scraper{
		endpoint:       cfg.Endpoint,
		logger:         logger,
		metricsBuilder: metricsBuilder,
	}
}
