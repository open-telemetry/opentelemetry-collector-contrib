// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package haproxyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver"

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver/internal/metadata"
)

var (
	showStatsCommand = []byte("show stats\n")
)

type scraper struct {
	cfg               *Config
	httpClient        *http.Client
	logger            *zap.Logger
	mb                *metadata.MetricsBuilder
	telemetrySettings component.TelemetrySettings
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {

	var records []map[string]string
	if u, notURLerr := url.Parse(s.cfg.Endpoint); notURLerr == nil && strings.HasPrefix(u.Scheme, "http") {

		resp, err := s.httpClient.Get(s.cfg.Endpoint + ";csv")
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		defer resp.Body.Close()
		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		records, err = s.readStats(buf)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
	} else {
		var d net.Dialer
		c, err := d.DialContext(ctx, "unix", s.cfg.Endpoint)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		defer func(c net.Conn) {
			_ = c.Close()
		}(c)
		_, err = c.Write(showStatsCommand)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		buf := make([]byte, 4096)
		_, err = c.Read(buf)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		records, err = s.readStats(buf)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
	}

	var scrapeErrors []error

	now := pcommon.NewTimestampFromTime(time.Now())
	for _, record := range records {
		if record["scur"] != "" {
			if err := s.mb.RecordHaproxySessionsCountDataPoint(now, record["scur"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["conn_rate"] != "" {
			if err := s.mb.RecordHaproxyConnectionsRateDataPoint(now, record["conn_rate"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["conn_tot"] != "" {
			if err := s.mb.RecordHaproxyConnectionsTotalDataPoint(now, record["conn_tot"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["lbtot"] != "" {
			if err := s.mb.RecordHaproxyServerSelectedTotalDataPoint(now, record["lbtot"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["bin"] != "" {
			if err := s.mb.RecordHaproxyBytesInputDataPoint(now, record["bin"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["bout"] != "" {
			if err := s.mb.RecordHaproxyBytesOutputDataPoint(now, record["bout"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["cli_abrt"] != "" {
			if err := s.mb.RecordHaproxyClientsCanceledDataPoint(now, record["cli_abrt"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_byp"] != "" {
			if err := s.mb.RecordHaproxyCompressionBypassDataPoint(now, record["comp_byp"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_in"] != "" {
			if err := s.mb.RecordHaproxyCompressionInputDataPoint(now, record["comp_in"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_out"] != "" {
			if err := s.mb.RecordHaproxyCompressionOutputDataPoint(now, record["comp_out"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_rsp"] != "" {
			if err := s.mb.RecordHaproxyCompressionCountDataPoint(now, record["comp_rsp"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["dreq"] != "" {
			if err := s.mb.RecordHaproxyRequestsDeniedDataPoint(now, record["dreq"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["dresp"] != "" {
			if err := s.mb.RecordHaproxyResponsesDeniedDataPoint(now, record["dresp"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["downtime"] != "" {
			if err := s.mb.RecordHaproxyDowntimeDataPoint(now, record["downtime"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["econ"] != "" {
			if err := s.mb.RecordHaproxyConnectionsErrorsDataPoint(now, record["econ"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["ereq"] != "" {
			if err := s.mb.RecordHaproxyRequestsErrorsDataPoint(now, record["ereq"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["eresp"] != "" && record["srv_abrt"] != "" {
			aborts := record["srv_abrt"]
			eresp := record["eresp"]
			abortsVal, err2 := strconv.ParseInt(aborts, 10, 64)
			if err2 != nil {
				scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse int64 for HaproxyResponsesErrors, value was %s: %w", aborts, err2))
			}
			erespVal, err2 := strconv.ParseInt(eresp, 10, 64)
			if err2 != nil {
				scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse int64 for HaproxyResponsesErrors, value was %s: %w", eresp, err2))
			}
			s.mb.RecordHaproxyResponsesErrorsDataPoint(now, abortsVal+erespVal)
		}
		if record["chkfail"] != "" {
			if err := s.mb.RecordHaproxyFailedChecksDataPoint(now, record["chkfail"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["wredis"] != "" {
			if err := s.mb.RecordHaproxyRequestsRedispatchedDataPoint(now, record["wredis"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["hrsp_1xx"] != "" {
			if err := s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_1xx"], metadata.AttributeStatusCode1xx); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["hrsp_2xx"] != "" {
			if err := s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_2xx"], metadata.AttributeStatusCode2xx); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["hrsp_3xx"] != "" {
			if err := s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_3xx"], metadata.AttributeStatusCode3xx); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["hrsp_4xx"] != "" {
			if err := s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_4xx"], metadata.AttributeStatusCode4xx); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["hrsp_5xx"] != "" {
			if err := s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_5xx"], metadata.AttributeStatusCode5xx); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["hrsp_other"] != "" {
			if err := s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_other"], metadata.AttributeStatusCodeOther); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["wretr"] != "" {
			if err := s.mb.RecordHaproxyConnectionsRetriesDataPoint(now, record["wretr"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["stot"] != "" {
			if err := s.mb.RecordHaproxySessionsTotalDataPoint(now, record["stot"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["qcur"] != "" {
			if err := s.mb.RecordHaproxyRequestsQueuedDataPoint(now, record["qcur"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["req_rate"] != "" {
			if err := s.mb.RecordHaproxyRequestsRateDataPoint(now, record["req_rate"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["ttime"] != "" {
			if err := s.mb.RecordHaproxySessionsAverageDataPoint(now, record["ttime"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["rate"] != "" {
			if err := s.mb.RecordHaproxySessionsRateDataPoint(now, record["rate"]); err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		rb := s.mb.NewResourceBuilder()
		rb.SetHaproxyProxyName(record["pxname"])
		rb.SetHaproxyServiceName(record["svname"])
		rb.SetHaproxyAddr(s.cfg.Endpoint)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	if len(scrapeErrors) > 0 {
		return s.mb.Emit(), scrapererror.NewPartialScrapeError(multierr.Combine(scrapeErrors...), len(scrapeErrors))
	}
	return s.mb.Emit(), nil
}

func (s *scraper) readStats(buf []byte) ([]map[string]string, error) {
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

func (s *scraper) start(_ context.Context, host component.Host) error {
	var err error
	s.httpClient, err = s.cfg.HTTPClientSettings.ToClient(host, s.telemetrySettings)
	return err
}

func newScraper(cfg *Config, settings receiver.CreateSettings) *scraper {
	return &scraper{
		logger:            settings.TelemetrySettings.Logger,
		mb:                metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		cfg:               cfg,
		telemetrySettings: settings.TelemetrySettings,
	}
}
