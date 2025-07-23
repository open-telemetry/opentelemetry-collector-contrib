// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"context"
	"crypto/tls"
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

// extractTLSInfo extracts TLS certificate information from the connection state
func extractTLSInfo(state *tls.ConnectionState) (issuer, commonName string, sans []any, timeLeft int64) {
	if state == nil || len(state.PeerCertificates) == 0 {
		return "", "", nil, 0
	}

	cert := state.PeerCertificates[0]
	issuer = cert.Issuer.String()
	commonName = cert.Subject.CommonName

	// Collect all SANs
	sans = make([]any, 0, len(cert.DNSNames)+len(cert.IPAddresses)+len(cert.URIs)+len(cert.EmailAddresses))
	for _, ip := range cert.IPAddresses {
		sans = append(sans, ip.String())
	}
	for _, uri := range cert.URIs {
		sans = append(sans, uri.String())
	}
	for _, dnsName := range cert.DNSNames {
		sans = append(sans, dnsName)
	}
	for _, emailAddress := range cert.EmailAddresses {
		sans = append(sans, emailAddress)
	}

	// Calculate time left until expiry
	currentTime := time.Now()
	timeLeftSeconds := cert.NotAfter.Sub(currentTime).Seconds()
	timeLeft = int64(timeLeftSeconds)

	return issuer, commonName, sans, timeLeft
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

			// Check if TLS metric is enabled and this is an HTTPS endpoint
			if h.cfg.Metrics.HttpcheckTLSCertRemaining.Enabled && resp != nil && resp.TLS != nil {
				// Extract TLS info directly from the HTTP response
				issuer, commonName, sans, timeLeft := extractTLSInfo(resp.TLS)
				if issuer != "" || commonName != "" || len(sans) > 0 {
					h.mb.RecordHttpcheckTLSCertRemainingDataPoint(
						now,
						timeLeft,
						h.cfg.Targets[targetIndex].Endpoint,
						issuer,
						commonName,
						sans,
					)
				}
			}
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
						int64(0), // Use 0 as status code when the class doesn't match
						req.Method,
						class,
					)
				}
			}
			mux.Unlock()
		}(client, idx)
	}

	wg.Wait()

	// Emit metrics and post-process to remove http.status_code when value is 0
	metrics := h.mb.Emit()
	removeStatusCodeForZeroValues(metrics)

	return metrics, nil
}

// removeStatusCodeForZeroValues removes the http.status_code attribute from httpcheck.status metrics
func removeStatusCodeForZeroValues(metrics pmetric.Metrics) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			ms := sm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if m.Name() == "httpcheck.status" {
					// Process sum data points
					if m.Type() == pmetric.MetricTypeSum {
						dps := m.Sum().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							// If the value is 0, remove the http.status_code attribute
							if dp.IntValue() == 0 {
								dp.Attributes().Remove("http.status_code")
							}
						}
					}
				}
			}
		}
	}
}

func newScraper(conf *Config, settings receiver.Settings) *httpcheckScraper {
	return &httpcheckScraper{
		cfg:      conf,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
	}
}
