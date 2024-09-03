// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver/internal/metadata"
)

var (
	errClientNotInit    = errors.New("client not initialized")
	httpResponseClasses = map[string]int{"1xx": 1, "2xx": 2, "3xx": 3, "4xx": 4, "5xx": 5}
)

type httpcheckScraper struct {
	clients  []*http.Client
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

// Custom transport to measure DNS resolution time
func newCustomTransport(insecureSkipVerify bool, dnsDuration *time.Duration) *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify, // Enable or disable cert verification as needed
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			ip := net.ParseIP(host)
			if ip != nil {
				return net.Dial(network, addr)
			}

			start := time.Now()
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			*dnsDuration = time.Since(start)
			if err != nil {
				return nil, err
			}

			var conn net.Conn
			for _, ip := range addrs {
				conn, err = net.Dial(network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					break
				}
			}

			if err != nil {
				return nil, err
			}
			return conn, nil
		},
	}
}

// start starts the scraper by creating a new HTTP Client on the scraper
func (h *httpcheckScraper) start(ctx context.Context, host component.Host) (err error) {
	for _, target := range h.cfg.Targets {
		client, clentErr := target.ToClient(ctx, host, h.settings)
		if clentErr != nil {
			err = multierr.Append(err, clentErr)
		}
		h.clients = append(h.clients, client)
	}
	return
}

// scrape connects to the endpoint and produces metrics based on the response
func (h *httpcheckScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if len(h.clients) == 0 {
		return pmetric.NewMetrics(), errClientNotInit
	}

	var wg sync.WaitGroup
	wg.Add(len(h.clients))
	var mux sync.Mutex

	for idx, client := range h.clients {
		go func(targetClient *http.Client, targetIndex int) {
			defer wg.Done()

			now := pcommon.NewTimestampFromTime(time.Now())
			dnsDuration := time.Duration(0)

			// Create a custom transport to measure DNS timing
			transport := newCustomTransport(h.cfg.Targets[targetIndex].ClientConfig.TLSSetting.InsecureSkipVerify, &dnsDuration)
			targetClient.Transport = transport

			req, err := http.NewRequestWithContext(ctx, h.cfg.Targets[targetIndex].Method, h.cfg.Targets[targetIndex].Endpoint, http.NoBody)
			if err != nil {
				h.settings.Logger.Error("failed to create request", zap.Error(err))
				return
			}

			start := time.Now()
			resp, err := targetClient.Do(req)
			mux.Lock()
			if dnsDuration != 0 {
				h.mb.RecordHttpcheckDnslookupDurationDataPoint(now, dnsDuration.Milliseconds(), h.cfg.Targets[targetIndex].Endpoint)
			}

			h.mb.RecordHttpcheckDurationDataPoint(now, time.Since(start).Milliseconds(), h.cfg.Targets[targetIndex].Endpoint)

			statusCode := 0
			if err != nil {
				h.mb.RecordHttpcheckErrorDataPoint(now, int64(1), h.cfg.Targets[targetIndex].Endpoint, err.Error())
			} else {
				statusCode = resp.StatusCode
			}

			for class, intVal := range httpResponseClasses {
				if statusCode/100 == intVal {
					h.mb.RecordHttpcheckStatusDataPoint(now, int64(1), h.cfg.Targets[targetIndex].Endpoint, int64(statusCode), req.Method, class)
				} else {
					h.mb.RecordHttpcheckStatusDataPoint(now, int64(0), h.cfg.Targets[targetIndex].Endpoint, int64(statusCode), req.Method, class)
				}
			}
			mux.Unlock()
		}(client, idx)
	}

	wg.Wait()

	return h.mb.Emit(), nil
}

func newScraper(conf *Config, settings receiver.Settings) *httpcheckScraper {
	return &httpcheckScraper{
		cfg:      conf,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
	}
}
