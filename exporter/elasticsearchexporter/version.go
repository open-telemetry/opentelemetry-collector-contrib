// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/zap"
)

type esInfo struct {
	Version struct {
		Number      string `json:"number"`
		BuildFlavor string `json:"build_flavor"`
	} `json:"version"`
}

func (es *esInfo) fetchESInfo(ctx context.Context, tp elastictransport.Interface) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody)
	if err != nil {
		return fmt.Errorf("creating es info request: %w", err)
	}

	resp, err := tp.Perform(req)
	if err != nil {
		return fmt.Errorf("fetching es info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("es info returned status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(es); err != nil {
		return fmt.Errorf("decoding es info: %w", err)
	}
	return nil
}

func logElasticsearchVersions(ctx context.Context, cfg *Config, set exporter.Settings, host component.Host) {
	infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	httpClient, err := cfg.ClientConfig.ToClient(ctx, host.GetExtensions(), set.TelemetrySettings)
	if err != nil {
		set.Logger.Warn("version detection: couldn't create Elasticsearch client", zap.Error(err))
		return
	}
	defer httpClient.CloseIdleConnections()

	endpoints, _ := cfg.endpoints()

	for _, endpoint := range endpoints {
		u, err := url.Parse(strings.TrimRight(endpoint, "/"))
		if err != nil {
			set.Logger.Warn("invalid Elasticsearch endpoint", zap.String("endpoint", endpoint), zap.Error(err))
			continue
		}
		maxRetries := defaultMaxRetries
		if cfg.Retry.MaxRetries != 0 {
			maxRetries = cfg.Retry.MaxRetries
		}
		tp, err := elastictransport.New(elastictransport.Config{
			URLs:          []*url.URL{u},
			Transport:     httpClient.Transport,
			DisableRetry:  !cfg.Retry.Enabled,
			MaxRetries:    maxRetries,
			RetryOnStatus: cfg.Retry.RetryOnStatus,
			RetryBackoff:  createElasticsearchBackoffFunc(&cfg.Retry),
		})
		if err != nil {
			set.Logger.Warn("version detection: failed to create transport", zap.String("endpoint", endpoint), zap.Error(err))
			continue
		}
		var info esInfo
		if err := info.fetchESInfo(infoCtx, tp); err != nil {
			set.Logger.Warn("failed to fetch Elasticsearch info", zap.String("endpoint", endpoint), zap.Error(err))
			continue
		}
		set.Logger.Info("Connected to Elasticsearch",
			zap.String("endpoint", endpoint),
			zap.String("version", info.Version.Number),
			zap.String("build_flavor", info.Version.BuildFlavor),
		)
	}
}
