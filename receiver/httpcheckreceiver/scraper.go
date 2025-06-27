// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"context"
	"errors"
	"io"
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

// start initializes the scraper by creating HTTP clients for each endpoint.
func (h *httpcheckScraper) start(ctx context.Context, host component.Host) (err error) {
	var expandedTargets []*targetConfig

	for _, target := range h.cfg.Targets {
		if target.Timeout == 0 {
			// Set a reasonable timeout to prevent hanging requests
			target.Timeout = 30 * time.Second
		}

		// Create a unified list of endpoints
		var allEndpoints []string
		if len(target.Endpoints) > 0 {
			allEndpoints = append(allEndpoints, target.Endpoints...) // Add all endpoints
		}
		if target.Endpoint != "" {
			allEndpoints = append(allEndpoints, target.Endpoint) // Add single endpoint
		}

		// Process each endpoint in the unified list
		for _, endpoint := range allEndpoints {
			client, clientErr := target.ToClient(ctx, host, h.settings)
			if clientErr != nil {
				h.settings.Logger.Error("failed to initialize HTTP client", zap.String("endpoint", endpoint), zap.Error(clientErr))
				err = multierr.Append(err, clientErr)
				continue
			}

			// Clone the target and assign the specific endpoint
			targetClone := *target
			targetClone.Endpoint = endpoint

			h.clients = append(h.clients, client)
			expandedTargets = append(expandedTargets, &targetClone) // Add the cloned target to expanded targets
		}
	}

	h.cfg.Targets = expandedTargets // Replace targets with expanded targets
	return
}

// scrape performs the HTTP checks and records metrics based on responses.
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

			req, err := http.NewRequestWithContext(
				ctx,
				h.cfg.Targets[targetIndex].Method,
				h.cfg.Targets[targetIndex].Endpoint, // Use the ClientConfig.Endpoint
				http.NoBody,
			)
			if err != nil {
				h.settings.Logger.Error("failed to create request", zap.Error(err))
				return
			}

			// Add headers to the request
			for key, value := range h.cfg.Targets[targetIndex].Headers {
				req.Header.Set(key, value.String()) // Convert configopaque.String to string
			}

			// Send the request and measure response time
			start := time.Now()
			resp, err := targetClient.Do(req)

			// Always close response body if it exists, even on error
			if resp != nil && resp.Body != nil {
				defer func() {
					// Drain the body to allow connection reuse
					_, _ = io.Copy(io.Discard, resp.Body)
					if closeErr := resp.Body.Close(); closeErr != nil {
						h.settings.Logger.Error("failed to close response body", zap.Error(closeErr))
					}
				}()
			}

			mux.Lock()
			h.mb.RecordHttpcheckDurationDataPoint(
				now,
				time.Since(start).Milliseconds(),
				h.cfg.Targets[targetIndex].Endpoint, // Use the correct endpoint
			)

			statusCode := 0
			if err != nil {
				h.mb.RecordHttpcheckErrorDataPoint(
					now,
					int64(1),
					h.cfg.Targets[targetIndex].Endpoint,
					err.Error(),
				)
			} else {
				statusCode = resp.StatusCode
			}

			// Record HTTP status class metrics
			for class, intVal := range httpResponseClasses {
				if statusCode/100 == intVal {
					h.mb.RecordHttpcheckStatusDataPoint(
						now,
						int64(1),
						h.cfg.Targets[targetIndex].Endpoint,
						int64(statusCode),
						req.Method,
						class,
					)
				} else {
					h.mb.RecordHttpcheckStatusDataPoint(
						now,
						int64(0),
						h.cfg.Targets[targetIndex].Endpoint,
						int64(statusCode),
						req.Method,
						class,
					)
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
