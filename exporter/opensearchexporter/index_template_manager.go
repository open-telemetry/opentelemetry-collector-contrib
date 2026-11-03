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
	ss4oTracesTemplateName = "ss4o-traces-index-template"
	ss4oLogsTemplateName   = "ss4o-logs-index-template"
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
// (date instead of date_nanos for timestamps in otel-v1 mode; expanded dotted
// attribute keys in ss4o mode); existing documents are unaffected. This matches
// the Data Prepper sink's posture for the same operation, which logs and retries
// on IOException rather than blocking pipeline initialization.
//
// The templates installed depend on the configured mapping mode: otel-v1 mode
// installs the Data Prepper-compatible templates, ss4o mode installs templates
// that map attribute bags as flat_object to avoid dotted-key mapping conflicts.
func (tm *templateManager) ensureTemplates(ctx context.Context, mode string) {
	switch mode {
	case MappingOTelV1.String():
		tm.ensureTemplate(ctx, otelV1SpanTemplateName, templates.OtelV1APMSpan)
		tm.ensureTemplate(ctx, otelV1LogsTemplateName, templates.OtelV1Logs)
	case MappingSS4O.String():
		tm.ensureTemplate(ctx, ss4oTracesTemplateName, templates.SS4OTraces)
		tm.ensureTemplate(ctx, ss4oLogsTemplateName, templates.SS4OLogs)
	}
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
