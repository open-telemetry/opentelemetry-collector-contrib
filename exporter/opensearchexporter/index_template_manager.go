// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/templates"
)

const (
	otelV1SpanTemplateName = "otel-v1-apm-span-index-template"
	otelV1LogsTemplateName = "otel-v1-logs-index-template"
)

type templateManager struct {
	client *opensearchapi.Client
	logger *zap.Logger
}

func newTemplateManager(client *opensearchapi.Client, logger *zap.Logger) *templateManager {
	return &templateManager{client: client, logger: logger}
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
