// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver/internal/metadata"
)

var (
	errClientNotInit    = errors.New("client not initialized")
	httpResponseClasses = map[string]int{"1xx": 1, "2xx": 2, "3xx": 3, "4xx": 4, "5xx": 5}
)

type httpcheckScraper struct {
	client   *http.Client
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

// start starts the scraper by creating a new HTTP Client on the scraper
func (h *httpcheckScraper) start(ctx context.Context, host component.Host) (err error) {
	h.client, err = h.cfg.ToClient(host, h.settings)
	return
}

// scrape connects to the endpoint and produces metrics based on the response
func (h *httpcheckScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if h.client == nil {
		return pmetric.NewMetrics(), errClientNotInit
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	req, err := http.NewRequestWithContext(ctx, h.cfg.Method, h.cfg.Endpoint, http.NoBody)
	if err != nil {
		return pmetric.Metrics{}, err
	}

	start := time.Now()
	resp, err := h.client.Do(req)
	h.mb.RecordHttpcheckDurationDataPoint(now, time.Since(start).Milliseconds(), h.cfg.Endpoint)

	statusCode := 0
	if err != nil {
		h.mb.RecordHttpcheckErrorDataPoint(now, int64(1), h.cfg.Endpoint, err.Error())
	} else {
		statusCode = resp.StatusCode
	}

	for class, intVal := range httpResponseClasses {
		if statusCode/100 == intVal {
			h.mb.RecordHttpcheckStatusDataPoint(now, int64(1), h.cfg.Endpoint, int64(statusCode), req.Method, class)
		} else {
			h.mb.RecordHttpcheckStatusDataPoint(now, int64(0), h.cfg.Endpoint, int64(statusCode), req.Method, class)
		}

	}

	return h.mb.Emit(), nil
}

func newScraper(conf *Config, settings receiver.CreateSettings) *httpcheckScraper {
	return &httpcheckScraper{
		cfg:      conf,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
	}
}
