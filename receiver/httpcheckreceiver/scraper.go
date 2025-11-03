// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"
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

// timingInfo holds timing information for different phases of HTTP request
type timingInfo struct {
	dnsStart      atomic.Int64
	dnsEnd        atomic.Int64
	connectStart  atomic.Int64
	connectEnd    atomic.Int64
	tlsStart      atomic.Int64
	tlsEnd        atomic.Int64
	writeStart    atomic.Int64
	writeEnd      atomic.Int64
	readStart     atomic.Int64
	readEnd       atomic.Int64
	requestStart  atomic.Int64
	responseStart atomic.Int64
}

// getDurations calculates the duration for each phase in milliseconds
func (t *timingInfo) getDurations() (dnsMs, tcpMs, tlsMs, requestMs, responseMs int64) {
	dnsStartNs := t.dnsStart.Load()
	dnsEndNs := t.dnsEnd.Load()
	if dnsStartNs != 0 && dnsEndNs != 0 {
		dnsMs = (dnsEndNs - dnsStartNs) / int64(time.Millisecond)
	}

	connectStartNs := t.connectStart.Load()
	connectEndNs := t.connectEnd.Load()
	if connectStartNs != 0 && connectEndNs != 0 {
		tcpMs = (connectEndNs - connectStartNs) / int64(time.Millisecond)
	}

	tlsStartNs := t.tlsStart.Load()
	tlsEndNs := t.tlsEnd.Load()
	if tlsStartNs != 0 && tlsEndNs != 0 {
		tlsMs = (tlsEndNs - tlsStartNs) / int64(time.Millisecond)
	}

	writeStartNs := t.writeStart.Load()
	writeEndNs := t.writeEnd.Load()
	if writeStartNs != 0 && writeEndNs != 0 {
		requestMs = (writeEndNs - writeStartNs) / int64(time.Millisecond)
	}

	readStartNs := t.readStart.Load()
	readEndNs := t.readEnd.Load()
	if readStartNs != 0 && readEndNs != 0 {
		responseMs = (readEndNs - readStartNs) / int64(time.Millisecond)
	}

	return dnsMs, tcpMs, tlsMs, requestMs, responseMs
}

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

// validateResponse performs response validation based on configured rules
func validateResponse(body []byte, validations []validationConfig) (passed, failed map[string]int) {
	passed = make(map[string]int)
	failed = make(map[string]int)

	for _, validation := range validations {
		// String matching validations
		if validation.Contains != "" {
			if strings.Contains(string(body), validation.Contains) {
				passed["contains"]++
			} else {
				failed["contains"]++
			}
		}

		if validation.NotContains != "" {
			if !strings.Contains(string(body), validation.NotContains) {
				passed["not_contains"]++
			} else {
				failed["not_contains"]++
			}
		}

		// JSON path validations
		if validation.JSONPath != "" {
			result := gjson.GetBytes(body, validation.JSONPath)
			if !result.Exists() {
				failed["json_path"]++
				continue
			}

			if validation.Equals != "" {
				if result.String() == validation.Equals {
					passed["json_path"]++
				} else {
					failed["json_path"]++
				}
			} else {
				// If no equals condition, just check existence
				passed["json_path"]++
			}
		}

		// Size validations
		bodySize := int64(len(body))
		if validation.MaxSize != nil {
			if bodySize <= *validation.MaxSize {
				passed["size"]++
			} else {
				failed["size"]++
			}
		}

		if validation.MinSize != nil {
			if bodySize >= *validation.MinSize {
				passed["size"]++
			} else {
				failed["size"]++
			}
		}

		// Regex validations
		if validation.Regex != "" {
			if matched, err := regexp.Match(validation.Regex, body); err == nil && matched {
				passed["regex"]++
			} else {
				failed["regex"]++
			}
		}
	}

	return passed, failed
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
	return err
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

			// Initialize timing info
			timing := &timingInfo{}

			// Create trace context for timing collection
			trace := &httptrace.ClientTrace{
				DNSStart: func(_ httptrace.DNSStartInfo) {
					timing.dnsStart.Store(time.Now().UnixNano())
				},
				DNSDone: func(_ httptrace.DNSDoneInfo) {
					timing.dnsEnd.Store(time.Now().UnixNano())
				},
				ConnectStart: func(_, _ string) {
					timing.connectStart.Store(time.Now().UnixNano())
				},
				ConnectDone: func(_, _ string, _ error) {
					timing.connectEnd.Store(time.Now().UnixNano())
				},
				TLSHandshakeStart: func() {
					timing.tlsStart.Store(time.Now().UnixNano())
				},
				TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
					timing.tlsEnd.Store(time.Now().UnixNano())
				},
				WroteRequest: func(_ httptrace.WroteRequestInfo) {
					timing.writeEnd.Store(time.Now().UnixNano())
				},
				GotFirstResponseByte: func() {
					now := time.Now().UnixNano()
					timing.responseStart.Store(now)
					timing.readStart.Store(now)
				},
			}

			// Create request with trace context and body
			var requestBody io.Reader = http.NoBody
			if h.cfg.Targets[targetIndex].Body != "" {
				requestBody = strings.NewReader(h.cfg.Targets[targetIndex].Body)
			}

			req, err := http.NewRequestWithContext(
				httptrace.WithClientTrace(ctx, trace),
				h.cfg.Targets[targetIndex].Method,
				h.cfg.Targets[targetIndex].Endpoint,
				requestBody,
			)
			if err != nil {
				h.settings.Logger.Error("failed to create request", zap.Error(err))
				return
			}

			// Add headers to the request
			for key, value := range h.cfg.Targets[targetIndex].Headers {
				req.Header.Set(key, value.String()) // Convert configopaque.String to string
			}

			// Set Content-Type header automatically if body is present, no Content-Type is set, and auto_content_type is enabled
			if h.cfg.Targets[targetIndex].Body != "" && req.Header.Get("Content-Type") == "" && h.cfg.Targets[targetIndex].AutoContentType {
				body := strings.TrimSpace(h.cfg.Targets[targetIndex].Body)
				switch {
				case strings.HasPrefix(body, "{") || strings.HasPrefix(body, "["):
					req.Header.Set("Content-Type", "application/json")
				case strings.Contains(h.cfg.Targets[targetIndex].Body, "="):
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				default:
					req.Header.Set("Content-Type", "text/plain")
				}
			}

			// Send the request and measure response time
			start := time.Now()
			startNs := start.UnixNano()
			timing.requestStart.Store(startNs)
			timing.writeStart.Store(startNs)
			resp, err := targetClient.Do(req)
			timing.readEnd.Store(time.Now().UnixNano())

			// Read response body for validation and metrics
			var responseBody []byte
			if resp != nil && resp.Body != nil {
				defer func() {
					if closeErr := resp.Body.Close(); closeErr != nil {
						h.settings.Logger.Error("failed to close response body", zap.Error(closeErr))
					}
				}()

				// Read the response body
				if body, readErr := io.ReadAll(resp.Body); readErr == nil {
					responseBody = body
				} else {
					h.settings.Logger.Error("failed to read response body", zap.Error(readErr))
				}
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
			// Record timing breakdown metrics
			dnsMs, tcpMs, tlsMs, requestMs, responseMs := timing.getDurations()
			endpoint := h.cfg.Targets[targetIndex].Endpoint

			h.mb.RecordHttpcheckDurationDataPoint(
				now,
				time.Since(start).Milliseconds(),
				endpoint,
			)
			// Record response size metric
			if len(responseBody) > 0 {
				h.mb.RecordHttpcheckResponseSizeDataPoint(now, int64(len(responseBody)), endpoint)
			}

			// Perform response validation if configured
			if len(h.cfg.Targets[targetIndex].Validations) > 0 && len(responseBody) > 0 {
				passed, failed := validateResponse(responseBody, h.cfg.Targets[targetIndex].Validations)

				// Record validation metrics
				for validationType, count := range passed {
					h.mb.RecordHttpcheckValidationPassedDataPoint(now, int64(count), endpoint, validationType)
				}

				for validationType, count := range failed {
					h.mb.RecordHttpcheckValidationFailedDataPoint(now, int64(count), endpoint, validationType)
				}
			}

			// Record detailed timing metrics if enabled
			// Always record timing metrics regardless of value for enabled metrics
			h.mb.RecordHttpcheckDNSLookupDurationDataPoint(now, dnsMs, endpoint)
			h.mb.RecordHttpcheckClientConnectionDurationDataPoint(now, tcpMs, endpoint, "tcp")
			h.mb.RecordHttpcheckTLSHandshakeDurationDataPoint(now, tlsMs, endpoint)
			h.mb.RecordHttpcheckClientRequestDurationDataPoint(now, requestMs, endpoint)
			h.mb.RecordHttpcheckResponseDurationDataPoint(now, responseMs, endpoint)

			// Check if TLS metric is enabled and this is an HTTPS endpoint
			if h.cfg.Metrics.HttpcheckTLSCertRemaining.Enabled && resp != nil && resp.TLS != nil {
				// Extract TLS info directly from the HTTP response
				issuer, commonName, sans, timeLeft := extractTLSInfo(resp.TLS)
				if issuer != "" || commonName != "" || len(sans) > 0 {
					h.mb.RecordHttpcheckTLSCertRemainingDataPoint(
						now,
						timeLeft,
						endpoint,
						issuer,
						commonName,
						sans,
					)
				}
			}

			statusCode := 0
			if err != nil {
				h.mb.RecordHttpcheckErrorDataPoint(
					now,
					int64(1),
					endpoint,
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
						endpoint,
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
