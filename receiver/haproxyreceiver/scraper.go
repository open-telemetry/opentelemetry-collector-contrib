// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package haproxyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver"

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

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
	endpoint string
	logger   *zap.Logger
	mb       *metadata.MetricsBuilder
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "unix", s.endpoint)
	if err != nil {
		return pmetric.NewMetrics(), err
	}
	defer func(c net.Conn) {
		_ = c.Close()
	}(c)
	records, err := s.readStats(c)
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	var scrapeErrors []error

	now := pcommon.NewTimestampFromTime(time.Now())
	for _, record := range records {
		rb := s.mb.NewResourceBuilder()
		rb.SetHaproxyProxyName(record["pxname"])
		rb.SetHaproxyServiceName(record["svname"])
		rb.SetHaproxyAddr(s.endpoint)
		rmb := s.mb.ResourceMetricsBuilder(rb.Emit())
		err = rmb.RecordHaproxySessionsCountDataPoint(now, record["scur"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		if record["conn_rate"] != "" {
			err = rmb.RecordHaproxyConnectionsRateDataPoint(now, record["conn_rate"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["conn_tot"] != "" {
			err = rmb.RecordHaproxyConnectionsTotalDataPoint(now, record["conn_tot"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["lbtot"] != "" {
			err = rmb.RecordHaproxyServerSelectedTotalDataPoint(now, record["lbtot"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		err = rmb.RecordHaproxyBytesInputDataPoint(now, record["bin"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = rmb.RecordHaproxyBytesOutputDataPoint(now, record["bout"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		if record["cli_abrt"] != "" {
			err = rmb.RecordHaproxyClientsCanceledDataPoint(now, record["cli_abrt"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_byp"] != "" {
			err = rmb.RecordHaproxyCompressionBypassDataPoint(now, record["comp_byp"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_in"] != "" {
			err = rmb.RecordHaproxyCompressionInputDataPoint(now, record["comp_in"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_out"] != "" {
			err = rmb.RecordHaproxyCompressionOutputDataPoint(now, record["comp_out"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["comp_rsp"] != "" {
			err = rmb.RecordHaproxyCompressionCountDataPoint(now, record["comp_rsp"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["dreq"] != "" {
			err = rmb.RecordHaproxyRequestsDeniedDataPoint(now, record["dreq"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["dresp"] != "" {
			err = rmb.RecordHaproxyResponsesDeniedDataPoint(now, record["dresp"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["downtime"] != "" {
			err = rmb.RecordHaproxyDowntimeDataPoint(now, record["downtime"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["econ"] != "" {
			err = rmb.RecordHaproxyConnectionsErrorsDataPoint(now, record["econ"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["ereq"] != "" {
			err = rmb.RecordHaproxyRequestsErrorsDataPoint(now, record["ereq"])
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
			rmb.RecordHaproxyResponsesErrorsDataPoint(now, abortsVal+erespVal)
		}
		if record["chkfail"] != "" {
			err = rmb.RecordHaproxyFailedChecksDataPoint(now, record["chkfail"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["wredis"] != "" {
			err = rmb.RecordHaproxyRequestsRedispatchedDataPoint(now, record["wredis"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		err = rmb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_1xx"], metadata.AttributeStatusCode1xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = rmb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_2xx"], metadata.AttributeStatusCode2xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = rmb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_3xx"], metadata.AttributeStatusCode3xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = rmb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_4xx"], metadata.AttributeStatusCode4xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = rmb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_5xx"], metadata.AttributeStatusCode5xx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		err = rmb.RecordHaproxyRequestsTotalDataPoint(now, record["hrsp_other"], metadata.AttributeStatusCodeOther)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		if record["wretr"] != "" {
			err = rmb.RecordHaproxyConnectionsRetriesDataPoint(now, record["wretr"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		err = rmb.RecordHaproxySessionsTotalDataPoint(now, record["stot"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
		if record["qcur"] != "" {
			err = rmb.RecordHaproxyRequestsQueuedDataPoint(now, record["qcur"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["req_rate"] != "" {
			err = rmb.RecordHaproxyRequestsRateDataPoint(now, record["req_rate"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		if record["ttime"] != "" {
			err = rmb.RecordHaproxySessionsAverageDataPoint(now, record["ttime"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
		err = rmb.RecordHaproxySessionsRateDataPoint(now, record["rate"])
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
	}

	if len(scrapeErrors) > 0 {
		return s.mb.Emit(), scrapererror.NewPartialScrapeError(multierr.Combine(scrapeErrors...), len(scrapeErrors))
	}
	return s.mb.Emit(), nil
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

func newScraper(cfg *Config, settings receiver.CreateSettings) *scraper {
	return &scraper{
		endpoint: cfg.Endpoint,
		logger:   settings.TelemetrySettings.Logger,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}
