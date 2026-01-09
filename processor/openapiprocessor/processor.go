// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor"

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// openAPIProcessor processes traces by matching URLs against OpenAPI paths.
type openAPIProcessor struct {
	config      *Config
	logger      *zap.Logger
	spec        atomic.Pointer[OpenAPISpec]
	peerService string

	// For background refresh
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// newOpenAPIProcessor creates a new OpenAPI processor instance.
func newOpenAPIProcessor(cfg *Config, logger *zap.Logger) (*openAPIProcessor, error) {
	spec, err := ParseOpenAPIFile(cfg.OpenAPIFile)
	if err != nil {
		return nil, err
	}

	// Determine peer service name
	peerService := cfg.PeerService
	if peerService == "" && spec.GetTitle() != "" {
		peerService = spec.GetTitle()
	}
	// Replace template placeholder if present
	peerService = strings.ReplaceAll(peerService, "${info.title}", spec.GetTitle())

	logger.Info("OpenAPI processor initialized",
		zap.String("openapi_file", cfg.OpenAPIFile),
		zap.String("peer_service", peerService),
		zap.Int("path_count", len(spec.GetPathTemplates())),
		zap.Strings("servers", spec.GetServers()),
		zap.Bool("use_server_url_matching", cfg.UseServerURLMatching),
		zap.Duration("refresh_interval", cfg.RefreshInterval),
	)

	op := &openAPIProcessor{
		config:      cfg,
		logger:      logger,
		peerService: peerService,
		stopChan:    make(chan struct{}),
	}
	op.spec.Store(spec)

	// Start background refresh if configured and using HTTP URL
	if cfg.RefreshInterval > 0 && isHTTPURL(cfg.OpenAPIFile) {
		op.startBackgroundRefresh()
	}

	return op, nil
}

// startBackgroundRefresh starts a goroutine that periodically refreshes the OpenAPI spec.
func (op *openAPIProcessor) startBackgroundRefresh() {
	op.wg.Add(1)
	go func() {
		defer op.wg.Done()
		ticker := time.NewTicker(op.config.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				op.refreshSpec()
			case <-op.stopChan:
				return
			}
		}
	}()
	op.logger.Info("Started background OpenAPI spec refresh",
		zap.Duration("interval", op.config.RefreshInterval),
	)
}

// refreshSpec fetches and updates the OpenAPI spec.
func (op *openAPIProcessor) refreshSpec() {
	newSpec, err := ParseOpenAPIFile(op.config.OpenAPIFile)
	if err != nil {
		op.logger.Warn("Failed to refresh OpenAPI spec",
			zap.Error(err),
			zap.String("openapi_file", op.config.OpenAPIFile),
		)
		return
	}

	op.spec.Store(newSpec)
	op.logger.Debug("Successfully refreshed OpenAPI spec",
		zap.Int("path_count", len(newSpec.GetPathTemplates())),
	)
}

// shutdown stops the background refresh goroutine.
func (op *openAPIProcessor) shutdown(ctx context.Context) error {
	close(op.stopChan)

	// Wait for background goroutine to finish with timeout
	done := make(chan struct{})
	go func() {
		op.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// processTraces processes all traces and adds url.template and peer.service attributes.
func (op *openAPIProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				op.processSpan(span)
			}
		}
	}
	return td, nil
}

// processSpan processes a single span and adds url.template and peer.service if applicable.
func (op *openAPIProcessor) processSpan(span ptrace.Span) {
	attrs := span.Attributes()

	// Check if we should skip existing attributes
	if !op.config.OverwriteExisting {
		if _, exists := attrs.Get(op.config.URLTemplateAttribute); exists {
			return
		}
	}

	// Get the URL from the configured attribute
	urlValue, exists := op.getURLPath(attrs)
	if !exists || urlValue == "" {
		return
	}

	// Strip query parameters if configured
	urlPath := urlValue
	if !op.config.IncludeQueryParams {
		urlPath = op.stripQueryParams(urlValue)
	}

	// If server URL matching is enabled, check if the URL matches any server
	pathToMatch := urlPath
	spec := op.spec.Load()
	if op.config.UseServerURLMatching {
		matchedServer, remainingPath, matched := spec.MatchServerURL(urlValue)
		if !matched {
			// No server URL matched - skip this span
			return
		}
		// Use the remaining path after the server URL for path matching
		pathToMatch = remainingPath
		if pathToMatch == "" {
			pathToMatch = "/"
		}
		// Strip query params from the remaining path if needed
		if !op.config.IncludeQueryParams {
			pathToMatch = op.stripQueryParams(pathToMatch)
		}
		op.logger.Debug("Matched server URL",
			zap.String("full_url", urlValue),
			zap.String("matched_server", matchedServer),
			zap.String("remaining_path", pathToMatch),
		)
	}

	// Match against OpenAPI paths
	template := spec.MatchPath(pathToMatch)
	if template == "" {
		return
	}

	// Set url.template attribute
	attrs.PutStr(op.config.URLTemplateAttribute, template)

	// Set peer.service attribute if configured
	if op.peerService != "" {
		if op.config.OverwriteExisting {
			attrs.PutStr(op.config.PeerServiceAttribute, op.peerService)
		} else if _, exists := attrs.Get(op.config.PeerServiceAttribute); !exists {
			attrs.PutStr(op.config.PeerServiceAttribute, op.peerService)
		}
	}
}

// getURLPath extracts the URL path from span attributes.
// It tries the configured attribute first, then falls back to common URL attributes.
func (op *openAPIProcessor) getURLPath(attrs pcommon.Map) (string, bool) {
	// Try the configured attribute
	if val, exists := attrs.Get(op.config.URLAttribute); exists {
		return val.Str(), true
	}

	// Fallback to other common URL attributes
	fallbackAttrs := []string{"http.url", "http.target", "url.path", "url.full"}
	for _, attr := range fallbackAttrs {
		if attr == op.config.URLAttribute {
			continue // Already tried
		}
		if val, exists := attrs.Get(attr); exists {
			return val.Str(), true
		}
	}

	return "", false
}

// stripQueryParams removes query parameters from a URL.
func (op *openAPIProcessor) stripQueryParams(urlStr string) string {
	// Try to parse as a full URL first
	if parsed, err := url.Parse(urlStr); err == nil {
		return parsed.Path
	}

	// If not a valid URL, try to strip query string manually
	if idx := strings.Index(urlStr, "?"); idx != -1 {
		return urlStr[:idx]
	}

	return urlStr
}
