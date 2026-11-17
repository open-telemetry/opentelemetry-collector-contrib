// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remoteconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/remoteconfig"

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

const telemetryPath = "/telemetry/proxy/api/v2/apmtelemetry"

// InstrumentationTelemetryConfig controls opaque, synchronous SDK forwarding.
type InstrumentationTelemetryConfig struct {
	Enabled               bool          `mapstructure:"enabled"`
	Endpoint              string        `mapstructure:"endpoint"`
	MaxRequestBytes       int64         `mapstructure:"max_request_bytes"`
	MaxInflightBytes      int64         `mapstructure:"max_inflight_bytes"`
	MaxConcurrentRequests int           `mapstructure:"max_concurrent_requests"`
	Timeout               time.Duration `mapstructure:"timeout"`
}

func DefaultInstrumentationTelemetryConfig() InstrumentationTelemetryConfig {
	return InstrumentationTelemetryConfig{MaxRequestBytes: 5 << 20, MaxInflightBytes: 25 << 20, MaxConcurrentRequests: 20, Timeout: 10 * time.Second}
}

func (c InstrumentationTelemetryConfig) intake(site string) string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return "https://instrumentation-telemetry-intake." + site + "/api/v2/apmtelemetry"
}

func (c InstrumentationTelemetryConfig) validate(site string) error {
	u, err := url.Parse(c.intake(site))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/api/v2/apmtelemetry" {
		return errors.New("instrumentation_telemetry.endpoint must be an HTTPS /api/v2/apmtelemetry URL without credentials, query or fragment")
	}
	if c.MaxRequestBytes <= 0 || c.MaxRequestBytes >= 1<<62 || c.MaxInflightBytes < c.MaxRequestBytes || c.MaxConcurrentRequests <= 0 || c.Timeout <= 0 {
		return errors.New("instrumentation_telemetry requires positive limits and timeout, and max_inflight_bytes >= max_request_bytes")
	}
	return nil
}

type instrumentationTelemetry struct {
	config   InstrumentationTelemetryConfig
	api      APIConfig
	backend  confighttp.ClientConfig
	client   *http.Client
	lifetime context.Context
	slots    chan struct{}
	bytes    atomic.Int64
}

func newInstrumentationTelemetry(config InstrumentationTelemetryConfig, api APIConfig, backend confighttp.ClientConfig) *instrumentationTelemetry {
	return &instrumentationTelemetry{config: config, api: api, backend: backend, slots: make(chan struct{}, config.MaxConcurrentRequests)}
}

func (t *instrumentationTelemetry) start(ctx context.Context, host component.Host, settings component.TelemetrySettings) error {
	// Do not instrument the forwarding client into the telemetry it forwards.
	settings.TracerProvider, settings.MeterProvider = nil, nil
	client, err := t.backend.ToClient(ctx, host.GetExtensions(), settings)
	if err != nil {
		return err
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	t.client, t.lifetime = client, ctx
	return nil
}

func (t *instrumentationTelemetry) close() {
	if t.client != nil {
		t.client.CloseIdleConnections()
	}
}

func (t *instrumentationTelemetry) reserve(n int64) bool {
	for {
		current := t.bytes.Load()
		if n > t.config.MaxInflightBytes-current {
			return false
		}
		if t.bytes.CompareAndSwap(current, current+n) {
			return true
		}
	}
}

func (t *instrumentationTelemetry) serveHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case t.slots <- struct{}{}:
		defer func() { <-t.slots }()
	default:
		telemetryUnavailable(w)
		return
	}
	if r.ContentLength > t.config.MaxRequestBytes {
		http.Error(w, "telemetry request too large", http.StatusRequestEntityTooLarge)
		return
	}
	reserved := r.ContentLength
	if reserved < 0 {
		reserved = t.config.MaxRequestBytes
	}
	if !t.reserve(reserved) {
		telemetryUnavailable(w)
		return
	}
	defer t.bytes.Add(-reserved)
	body, err := io.ReadAll(io.LimitReader(r.Body, reserved+1))
	if err != nil || int64(len(body)) > reserved {
		http.Error(w, "invalid or oversized telemetry body", http.StatusRequestEntityTooLarge)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), t.config.Timeout)
	defer cancel()
	stop := context.AfterFunc(t.lifetime, cancel)
	defer stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.config.intake(t.api.Site), bytes.NewReader(body))
	if err != nil {
		http.Error(w, "telemetry destination unavailable", http.StatusBadGateway)
		return
	}
	for key, values := range r.Header {
		name := strings.ToLower(key)
		allowed := name == "content-type" || name == "content-encoding" || strings.HasPrefix(name, "dd-telemetry-") || strings.HasPrefix(name, "dd-client-library-") || name == "datadog-container-id" || name == "datadog-entity-id" || name == "datadog-external-env" || name == "datadog-runtime-id"
		if !allowed || connectionHeader(r.Header, key) {
			continue
		}
		req.Header[key] = append([]string(nil), values...)
	}
	req.Header.Set("DD-API-KEY", string(t.api.Key))
	response, err := t.client.Do(req)
	if err != nil {
		http.Error(w, "telemetry intake unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if hint := response.Header.Get("Retry-After"); hint != "" {
		w.Header().Set("Retry-After", hint)
	}
	code := response.StatusCode
	if code == http.StatusNotFound || code >= 300 && code < 400 {
		code = http.StatusBadGateway
	}
	w.WriteHeader(code)
}

func connectionHeader(headers http.Header, key string) bool {
	for _, value := range headers.Values("Connection") {
		for name := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(name), key) {
				return true
			}
		}
	}
	return false
}

func telemetryUnavailable(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "1")
	http.Error(w, "telemetry forwarding is saturated", http.StatusServiceUnavailable)
}
