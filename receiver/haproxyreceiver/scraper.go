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
		err := s.mb.RecordHaproxySessionsCountDataPoint(now, record["scur"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		if record["conn_rate"] != "" {
			err = s.mb.RecordHaproxyConnectionsRateDataPoint(now, record["conn_rate"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["conn_tot"] != "" {
			err = s.mb.RecordHaproxyConnectionsTotalDataPoint(now, record["conn_tot"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["lbtot"] != "" {
			err = s.mb.RecordHaproxyServerSelectedTotalDataPoint(now, record["lbtot"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		err = s.mb.RecordHaproxyBytesInputDataPoint(now, record["bin"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = s.mb.RecordHaproxyBytesOutputDataPoint(now, record["bout"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		if record["cli_abrt"] != "" {
			err = s.mb.RecordHaproxyClientsCanceledDataPoint(now, record["cli_abrt"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_byp"] != "" {
			err = s.mb.RecordHaproxyCompressionBypassDataPoint(now, record["comp_byp"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_in"] != "" {
			err = s.mb.RecordHaproxyCompressionInputDataPoint(now, record["comp_in"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_out"] != "" {
			err = s.mb.RecordHaproxyCompressionOutputDataPoint(now, record["comp_out"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_rsp"] != "" {
			err = s.mb.RecordHaproxyCompressionCountDataPoint(now, record["comp_rsp"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["dreq"] != "" {
			err = s.mb.RecordHaproxyRequestsDeniedDataPoint(now, record["dreq"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["dresp"] != "" {
			err = s.mb.RecordHaproxyResponsesDeniedDataPoint(now, record["dresp"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["downtime"] != "" {
			err = s.mb.RecordHaproxyDowntimeDataPoint(now, record["downtime"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["econ"] != "" {
			err = s.mb.RecordHaproxyConnectionsErrorsDataPoint(now, record["econ"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["ereq"] != "" {
			err = s.mb.RecordHaproxyRequestsErrorsDataPoint(now, record["ereq"])
			if err != nil {
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
			err = s.mb.RecordHaproxyFailedChecksDataPoint(now, record["chkfail"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["wredis"] != "" {
			err = s.mb.RecordHaproxyRequestsRedispatchedDataPoint(now, record["wredis"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		err = s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_1xx"], metadata.AttributeStatusCode1xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_2xx"], metadata.AttributeStatusCode2xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_3xx"], metadata.AttributeStatusCode3xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_4xx"], metadata.AttributeStatusCode4xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_5xx"], metadata.AttributeStatusCode5xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = s.mb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_other"], metadata.AttributeStatusCodeOther)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		if record["wretr"] != "" {
			err = s.mb.RecordHaproxyConnectionsRetriesDataPoint(now, record["wretr"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		err = s.mb.RecordHaproxySessionsTotalDataPoint(now, record["stot"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		if record["qcur"] != "" {
			err = s.mb.RecordHaproxyRequestsQueuedDataPoint(now, record["qcur"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["req_rate"] != "" {
			err = s.mb.RecordHaproxyRequestsRateDataPoint(now, record["req_rate"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["ttime"] != "" {
			err = s.mb.RecordHaproxySessionsAverageDataPoint(now, record["ttime"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		err = s.mb.RecordHaproxySessionsRateDataPoint(now, record["rate"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
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
