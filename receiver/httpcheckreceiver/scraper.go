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
	clients   []*http.Client
	sequences map[string]*sequence
	cfg       *Config
	settings  component.TelemetrySettings
	mb        *metadata.MetricsBuilder
	mx        sync.Mutex
}

type sequence struct {
	config  *sequenceConfig
	results map[string]string
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

	h.sequences = make(map[string]*sequence)
	for _, seqConfig := range h.cfg.Sequences {
		h.sequences[seqConfig.Name] = &sequence{
			config:  seqConfig,
			results: make(map[string]string),
		}
	}
	return
}

func (h *httpcheckScraper) scrapeTarget(ctx context.Context, targetConfig *targetConfig, targetIndex int) error {
	// setup telemetry
	now := pcommon.NewTimestampFromTime(time.Now())
	start := time.Now()
	// build request
	req, err := http.NewRequestWithContext(ctx, h.cfg.Targets[targetIndex].Method, h.cfg.Targets[targetIndex].Endpoint, http.NoBody)
	if err != nil {
		h.settings.Logger.Error("failed to create request", zap.Error(err))
		return err
	}
	// execute request
	targetClient := h.clients[targetIndex]
	resp, err := targetClient.Do(req)
	if err != nil {
		h.settings.Logger.Error("failed to execute request", zap.Error(err))
	}
	// record metrics
	h.mx.Lock()
	defer h.mx.Unlock()
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
	return nil
}

func (h *httpcheckScraper) scrapeSequence(ctx context.Context, seqConfig *sequenceConfig) {
	seq := h.sequences[seqConfig.Name]
	for _, step := range seqConfig.Steps {
		target := h.cfg.Targets[step.TargetIndex]
		client := h.clients[step.TargetIndex]

		req, err := http.NewRequestWithContext(ctx, target.Method, target.Endpoint, http.NoBody)
		if err != nil {
			h.settings.Logger.Error("failed to create request for sequence", zap.Error(err))
			return
		}

		start := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(start)
		now := pcommon.NewTimestampFromTime(time.Now())

		func() {
			h.mx.Lock()
			defer h.mx.Unlock()
			h.mb.RecordHttpcheckDurationDataPoint(now, duration.Milliseconds(), target.Endpoint)

			if err != nil {
				h.mb.RecordHttpcheckErrorDataPoint(now, int64(1), target.Endpoint, err.Error())
			} else {
				statusCode := resp.StatusCode
				for class, intVal := range httpResponseClasses {
					if statusCode/100 == intVal {
						h.mb.RecordHttpcheckStatusDataPoint(now, int64(1), target.Endpoint, int64(statusCode), req.Method, class)
					} else {
						h.mb.RecordHttpcheckStatusDataPoint(now, int64(0), target.Endpoint, int64(statusCode), req.Method, class)
					}
				}

				// Store response for future reference
				if resp.Body != nil {
					body, _ := io.ReadAll(resp.Body)
					seq.results[step.ResponseRef] = string(body)
					resp.Body.Close()
				}
			}
		}()
	}
}

func (h *httpcheckScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if h.clients == nil || len(h.clients) == 0 {
		return pmetric.NewMetrics(), errClientNotInit
	}

	var wg sync.WaitGroup

	// Scrape individual targets
	for idx, target := range h.cfg.Targets {
		wg.Add(1)
		go func(targetIndex int, targetConfig *targetConfig) {
			defer wg.Done()
			h.scrapeTarget(ctx, targetConfig, targetIndex)
		}(idx, target)
	}

	// Scrape sequences
	for _, seq := range h.sequences {
		wg.Add(1)
		go func(sequenceConfig *sequenceConfig) {
			defer wg.Done()
			h.scrapeSequence(ctx, sequenceConfig)
		}(seq.config)
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
