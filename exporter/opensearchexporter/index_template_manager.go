// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/templates"
)

const (
	otelV1SpanTemplateName = "otel-v1-apm-span-index-template"
	otelV1LogsTemplateName = "otel-v1-logs-index-template"

	// Base index/alias names for otel-v1 mode. These match the index_patterns in the
	// embedded templates (otel-v1-apm-span*, otel-v1-logs*) and are used as the write alias
	// for ISM rollover.
	otelV1SpanIndexAlias = "otel-v1-apm-span"
	otelV1LogsIndexAlias = "otel-v1-logs"
)

type templateManager struct {
	client *opensearchapi.Client
	logger *zap.Logger

	// customOverlay is an optional JSON document deep-merged over each built-in template
	// body before creation. Empty when no custom index template file is configured.
	customOverlay string
}

func newTemplateManager(client *opensearchapi.Client, logger *zap.Logger, customTemplateFile string) *templateManager {
	tm := &templateManager{client: client, logger: logger}
	if customTemplateFile != "" {
		data, err := os.ReadFile(customTemplateFile)
		if err != nil {
			// Best-effort: fall back to the built-in templates so a bad path does not
			// block startup. Validation only checks that the option is used in a valid
			// mode, not that the file exists, matching the exporter's other file options.
			logger.Warn("Failed to read custom index template file; using built-in templates",
				zap.String("file", customTemplateFile), zap.Error(err))
		} else {
			tm.customOverlay = string(data)
		}
	}
	return tm
}

// ensureTemplates is best-effort: it logs and returns on transient cluster
// errors rather than failing the exporter Start(). A failure means OpenSearch's
// dynamic mapping will pick up types from the first indexed document
// (date instead of date_nanos for timestamps); existing documents are
// unaffected. This matches the Data Prepper sink's posture for the same
// operation, which logs and retries on IOException rather than blocking
// pipeline initialization.
func (tm *templateManager) ensureTemplates(ctx context.Context) {
	tm.ensureTemplate(ctx, otelV1SpanTemplateName, templates.OtelV1APMSpan)
	tm.ensureTemplate(ctx, otelV1LogsTemplateName, templates.OtelV1Logs)
}

func (tm *templateManager) ensureTemplate(ctx context.Context, name, body string) {
	// Apply the optional custom overlay (e.g. to uplift attributes to typed fields or keep
	// them un-indexed) on top of the built-in template. On merge failure, fall back to the
	// built-in body rather than dropping the template entirely.
	if tm.customOverlay != "" {
		merged, err := mergeTemplateBody(body, tm.customOverlay)
		if err != nil {
			tm.logger.Warn("Failed to merge custom index template file; using built-in template",
				zap.String("template", name), zap.Error(err))
		} else {
			body = merged
		}
	}

	// Check if template exists
	existsReq := opensearchapi.IndexTemplateExistsReq{IndexTemplate: name}
	_, err := tm.client.IndexTemplate.Exists(ctx, existsReq)
	if err == nil {
		// Template exists, skip creation
		tm.logger.Debug("Index template already exists, skipping creation", zap.String("template", name))
		return
	}

	// Create template
	createReq := opensearchapi.IndexTemplateCreateReq{
		IndexTemplate: name,
		Body:          strings.NewReader(body),
	}
	_, createErr := tm.client.IndexTemplate.Create(ctx, createReq)
	if createErr != nil {
		tm.logger.Warn("Failed to create index template; falling back to dynamic mapping for this index. "+
			"Timestamp fields may be inferred as `date` (millisecond) instead of `date_nanos` until the template is installed.",
			zap.String("template", name), zap.Error(createErr))
		return
	}
	tm.logger.Info("Created index template", zap.String("template", name))
}

// mergeTemplateBody deep-merges overlay JSON over base JSON. Object values are merged
// recursively; array and scalar values in the overlay replace those in the base. This lets a
// user add or override mappings (e.g. typed properties or dynamic_templates) without restating
// the whole otel-v1 template.
func mergeTemplateBody(base, overlay string) (string, error) {
	var baseMap map[string]any
	if err := json.Unmarshal([]byte(base), &baseMap); err != nil {
		return "", fmt.Errorf("parsing built-in template: %w", err)
	}
	var overlayMap map[string]any
	if err := json.Unmarshal([]byte(overlay), &overlayMap); err != nil {
		return "", fmt.Errorf("parsing custom index template file: %w", err)
	}
	merged, err := json.Marshal(deepMerge(baseMap, overlayMap))
	if err != nil {
		return "", err
	}
	return string(merged), nil
}

// deepMerge recursively merges overlay into base and returns base. When a key holds an object
// in both, the objects are merged; otherwise the overlay value wins.
func deepMerge(base, overlay map[string]any) map[string]any {
	for k, ov := range overlay {
		if bv, ok := base[k]; ok {
			bMap, bIsMap := bv.(map[string]any)
			oMap, oIsMap := ov.(map[string]any)
			if bIsMap && oIsMap {
				base[k] = deepMerge(bMap, oMap)
				continue
			}
		}
		base[k] = ov
	}
	return base
}
