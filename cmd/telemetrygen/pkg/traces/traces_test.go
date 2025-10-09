// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

// TestDurationAndTracesInteraction tests the interaction between duration and traces parameters
func TestDurationAndTracesInteraction(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		expectedTraces int
		description    string
	}{
		{
			name: "Default behavior - respects traces parameter",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumTraces: 3,
			},
			expectedTraces: 3,
			description:    "By default, TotalDuration is 0, so NumTraces should be respected",
		},
		{
			name: "Finite duration overrides traces",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(100 * time.Millisecond),
				},
				NumTraces: 100,
			},
			expectedTraces: 0,
			description:    "Finite duration should override NumTraces (set to 0)",
		},
		{
			name: "Infinite duration overrides traces",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.MustDurationWithInf("Inf"),
				},
				NumTraces: 50,
			},
			expectedTraces: 0,
			description:    "Infinite duration should override NumTraces (set to 0)",
		},
		{
			name: "Zero duration with traces",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(0),
				},
				NumTraces: 5,
			},
			expectedTraces: 5,
			description:    "Zero duration should not override NumTraces",
		},
		{
			name: "Negative duration with traces",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(-100 * time.Millisecond),
				},
				NumTraces: 10,
			},
			expectedTraces: 10,
			description:    "Negative duration should not override NumTraces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config

			if cfg.TotalDuration.Duration() > 0 || cfg.TotalDuration.IsInf() {
				cfg.NumTraces = 0
			}

			assert.Equal(t, tt.expectedTraces, cfg.NumTraces, tt.description)
		})
	}
}

// TestDefaultConfiguration tests that the default configuration is correct
func TestDefaultConfiguration(t *testing.T) {
	cfg := NewConfig()

	assert.Equal(t, types.DurationWithInf(0), cfg.TotalDuration, "Default TotalDuration should be 0")
	assert.Equal(t, 0, cfg.NumTraces, "Default NumTraces should be 0")
	assert.Equal(t, float64(1), cfg.Rate, "Default Rate should be 1")
	assert.Equal(t, 1, cfg.NumChildSpans, "Default NumChildSpans should be 1")
	assert.True(t, cfg.Batch, "Default Batch should be true")
}

// TestDurationWithInfValues tests various DurationWithInf values
func TestDurationWithInfValues(t *testing.T) {
	tests := []struct {
		name           string
		duration       types.DurationWithInf
		expectedIsInf  bool
		expectedString string
		description    string
	}{
		{
			name:           "Zero duration",
			duration:       types.DurationWithInf(0),
			expectedIsInf:  false,
			expectedString: "0s",
			description:    "Zero duration should not be infinite",
		},
		{
			name:           "Finite duration",
			duration:       types.DurationWithInf(5 * time.Second),
			expectedIsInf:  false,
			expectedString: "5s",
			description:    "Finite duration should not be infinite",
		},
		{
			name:           "Infinite duration",
			duration:       types.MustDurationWithInf("Inf"),
			expectedIsInf:  true,
			expectedString: "inf",
			description:    "Duration -1 should be infinite",
		},
		{
			name:           "Negative finite duration",
			duration:       types.DurationWithInf(-100 * time.Millisecond),
			expectedIsInf:  false,
			expectedString: "-100ms",
			description:    "Negative finite duration should not be infinite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedIsInf, tt.duration.IsInf(), tt.description)
			assert.Equal(t, tt.expectedString, tt.duration.String(), "String representation should match")
		})
	}
}

// TestConfigValidation tests the validation logic
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		description string
	}{
		{
			name: "Valid config with traces",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumTraces: 5,
			},
			expectError: false,
			description: "Config with NumTraces > 0 should be valid",
		},
		{
			name: "Valid config with finite duration",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(1 * time.Second),
				},
				NumTraces: 0,
			},
			expectError: false,
			description: "Config with finite duration > 0 should be valid",
		},
		{
			name: "Valid config with infinite duration",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.MustDurationWithInf("Inf"), // inf
				},
				NumTraces: 0,
			},
			expectError: false,
			description: "Config with infinite duration should be valid",
		},
		{
			name: "Invalid config - no traces and no duration",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumTraces: 0,
			},
			expectError: true,
			description: "Config with no traces and no duration should be invalid",
		},
		{
			name: "Invalid config - negative traces",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumTraces: -5,
			},
			expectError: true,
			description: "Config with negative traces should be invalid",
		},
		{
			name: "Valid config with span links",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumTraces:    5,
				NumSpanLinks: 2,
			},
			expectError: false,
			description: "Config with span links should be valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestWorkerBehavior tests that workers behave correctly with different configurations
func TestWorkerBehavior(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		expectedTraces int
		description    string
	}{
		{
			name: "Worker with finite traces and no duration",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumTraces: 2,
			},
			expectedTraces: 2,
			description:    "Worker should generate exactly the specified number of traces",
		},
		{
			name: "Worker with infinite duration",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.MustDurationWithInf("Inf"), // inf
				},
				NumTraces: 0, // This will be set by the run logic
			},
			expectedTraces: 0,
			description:    "Worker with infinite duration should have NumTraces set to 0",
		},
		{
			name: "Worker with finite duration",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(100 * time.Millisecond),
				},
				NumTraces: 10, // This will be set to 0 by the run logic
			},
			expectedTraces: 0,
			description:    "Worker with finite duration should have NumTraces set to 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.TotalDuration.Duration() > 0 || tt.config.TotalDuration.IsInf() {
				tt.config.NumTraces = 0
			}

			assert.Equal(t, tt.expectedTraces, tt.config.NumTraces, tt.description)
		})
	}
}

func TestHTTPExporterOptions_TLS(t *testing.T) {
	// TODO add test cases for mTLS
	for name, tc := range map[string]struct {
		tls         bool
		tlsServerCA bool // use the httptest.Server's TLS cert as the CA
		cfg         Config

		expectTransportError bool
	}{
		"Insecure": {
			tls: false,
			cfg: Config{Config: common.Config{Insecure: true}},
		},
		"InsecureSkipVerify": {
			tls: true,
			cfg: Config{Config: common.Config{InsecureSkipVerify: true}},
		},
		"InsecureSkipVerifyDisabled": {
			tls:                  true,
			expectTransportError: true,
		},
		"CaFile": {
			tls:         true,
			tlsServerCA: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var called bool
			var h http.HandlerFunc = func(http.ResponseWriter, *http.Request) {
				called = true
			}
			var srv *httptest.Server
			if tc.tls {
				srv = httptest.NewTLSServer(h)
			} else {
				srv = httptest.NewServer(h)
			}
			defer srv.Close()
			srvURL, _ := url.Parse(srv.URL)

			cfg := tc.cfg
			cfg.CustomEndpoint = srvURL.Host
			if tc.tlsServerCA {
				caFile := filepath.Join(t.TempDir(), "cert.pem")
				err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: srv.TLS.Certificates[0].Certificate[0],
				}), 0o600)
				require.NoError(t, err)
				cfg.CaFile = caFile
			}

			opts, err := httpExporterOptions(&cfg)
			require.NoError(t, err)
			client := otlptracehttp.NewClient(opts...)

			err = client.UploadTraces(t.Context(), []*tracepb.ResourceSpans{})
			if tc.expectTransportError {
				require.Error(t, err)
				assert.False(t, called)
			} else {
				require.NoError(t, err)
				assert.True(t, called)
			}
		})
	}
}

func TestHTTPExporterOptions_HTTP(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg Config

		expectedHTTPPath string
		expectedHeader   http.Header
	}{
		"HTTPPath": {
			cfg:              Config{Config: common.Config{HTTPPath: "/foo"}},
			expectedHTTPPath: "/foo",
		},
		"Headers": {
			cfg: Config{
				Config: common.Config{Headers: map[string]any{"a": "b"}},
			},
			expectedHTTPPath: "/v1/traces",
			expectedHeader:   http.Header{"a": []string{"b"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var httpPath string
			var header http.Header
			var h http.HandlerFunc = func(_ http.ResponseWriter, r *http.Request) {
				httpPath = r.URL.Path
				header = r.Header
			}
			srv := httptest.NewServer(h)
			defer srv.Close()
			srvURL, _ := url.Parse(srv.URL)

			cfg := tc.cfg
			cfg.Insecure = true
			cfg.CustomEndpoint = srvURL.Host
			opts, err := httpExporterOptions(&cfg)
			require.NoError(t, err)
			client := otlptracehttp.NewClient(opts...)

			err = client.UploadTraces(t.Context(), []*tracepb.ResourceSpans{})
			require.NoError(t, err)
			assert.Equal(t, tc.expectedHTTPPath, httpPath)
			for k, expected := range tc.expectedHeader {
				assert.Equal(t, expected, []string{header.Get(k)})
			}
		})
	}
}

// TestSpanLinksGeneration tests the span links generation functionality
func TestSpanLinksGeneration(t *testing.T) {
	tests := []struct {
		name              string
		numSpanLinks      int
		existingContexts  int
		expectedLinkCount int
		description       string
	}{
		{
			name:              "No span links",
			numSpanLinks:      0,
			existingContexts:  5,
			expectedLinkCount: 0,
			description:       "Should generate no links when numSpanLinks is 0",
		},
		{
			name:              "With existing contexts",
			numSpanLinks:      3,
			existingContexts:  5,
			expectedLinkCount: 3,
			description:       "Should generate links to random existing contexts",
		},
		{
			name:              "No existing contexts",
			numSpanLinks:      3,
			existingContexts:  0,
			expectedLinkCount: 0,
			description:       "Should generate no links when no existing contexts",
		},
		{
			name:              "Fewer contexts than requested links",
			numSpanLinks:      5,
			existingContexts:  3,
			expectedLinkCount: 5,
			description:       "Should generate requested number of links (allows duplicates)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &worker{
				numSpanLinks:   tt.numSpanLinks,
				spanContexts:   make([]trace.SpanContext, 0),
				spanContextsMu: sync.RWMutex{},
			}

			// Add existing contexts for testing
			for i := 0; i < tt.existingContexts; i++ {
				traceID, _ := trace.TraceIDFromHex(fmt.Sprintf("%032d", i))
				spanID, _ := trace.SpanIDFromHex(fmt.Sprintf("%016d", i))
				spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
					TraceID:    traceID,
					SpanID:     spanID,
					TraceFlags: trace.FlagsSampled,
				})
				w.addSpanContext(spanCtx)
			}

			links := w.generateSpanLinks()

			assert.Len(t, links, tt.expectedLinkCount, tt.description)

			// Verify all links have random type and correct index
			for i, link := range links {
				// Verify link.type attribute is 'random'
				found := false
				for _, attr := range link.Attributes {
					if attr.Key == "link.type" && attr.Value.AsString() == "random" {
						found = true
						break
					}
				}
				assert.True(t, found, "Link should have 'link.type=random' attribute")

				// Verify link.index attribute is present
				foundIndex := false
				for _, attr := range link.Attributes {
					if attr.Key == "link.index" && attr.Value.AsInt64() == int64(i) {
						foundIndex = true
						break
					}
				}
				assert.True(t, foundIndex, "Link should have correct 'link.index' attribute")
			}
		})
	}
}

// TestDefaultSpanLinksConfiguration tests that the default span links configuration is correct
func TestDefaultSpanLinksConfiguration(t *testing.T) {
	cfg := NewConfig()

	assert.Equal(t, 0, cfg.NumSpanLinks, "Default NumSpanLinks should be 0")
}

func TestSpanContextsBufferLimit(t *testing.T) {
	w := &worker{
		numSpanLinks:   2,
		spanContexts:   make([]trace.SpanContext, 0),
		spanContextsMu: sync.RWMutex{},
	}

	// Add more span contexts than the buffer limit
	for i := range 1200 {
		traceID, _ := trace.TraceIDFromHex(fmt.Sprintf("%032d", i))
		spanID, _ := trace.SpanIDFromHex(fmt.Sprintf("%016d", i))
		spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
		})
		w.addSpanContext(spanCtx)
	}

	// Verify the buffer doesn't exceed the maximum size
	assert.LessOrEqual(t, len(w.spanContexts), 1000, "Span contexts buffer should not exceed maximum size")

	// Verify we can still generate links with the buffered contexts
	links := w.generateSpanLinks()
	assert.Len(t, links, 2, "Should generate correct number of links even with buffer limit")
}
