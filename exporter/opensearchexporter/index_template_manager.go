// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.uber.org/zap"
)

const (
	otelV1SpanTemplateName = "otel-v1-apm-span-index-template"
	otelV1LogsTemplateName = "otel-v1-logs-index-template"

	otelV1SpanTemplateBody = `{
  "index_patterns": ["otel-v1-apm-span*"],
  "template": {
    "mappings": {
      "date_detection": false,
      "_source": { "enabled": true },
      "dynamic_templates": [
        { "long_resource_attributes": { "mapping": { "type": "long" }, "path_match": "resource.attributes.*", "match_mapping_type": "long" } },
        { "double_resource_attributes": { "mapping": { "type": "double" }, "path_match": "resource.attributes.*", "match_mapping_type": "double" } },
        { "string_resource_attributes": { "mapping": { "type": "keyword", "ignore_above": 256 }, "path_match": "resource.attributes.*", "match_mapping_type": "string" } },
        { "long_scope_attributes": { "mapping": { "type": "long" }, "path_match": "instrumentationScope.attributes.*", "match_mapping_type": "long" } },
        { "double_scope_attributes": { "mapping": { "type": "double" }, "path_match": "instrumentationScope.attributes.*", "match_mapping_type": "double" } },
        { "string_scope_attributes": { "mapping": { "type": "keyword", "ignore_above": 256 }, "path_match": "instrumentationScope.attributes.*", "match_mapping_type": "string" } },
        { "long_attributes": { "mapping": { "type": "long" }, "path_match": "attributes.*", "match_mapping_type": "long" } },
        { "double_attributes": { "mapping": { "type": "double" }, "path_match": "attributes.*", "match_mapping_type": "double" } },
        { "string_attributes": { "mapping": { "type": "keyword", "ignore_above": 256 }, "path_match": "attributes.*", "match_mapping_type": "string" } }
      ],
      "properties": {
        "droppedAttributesCount": { "type": "integer" },
        "instrumentationScope": { "properties": { "droppedAttributesCount": { "type": "integer" }, "schemaUrl": { "type": "keyword", "ignore_above": 256 }, "name": { "type": "keyword", "ignore_above": 128 }, "version": { "type": "keyword", "ignore_above": 64 } } },
        "resource": { "properties": { "droppedAttributesCount": { "type": "integer" }, "schemaUrl": { "type": "keyword", "ignore_above": 256 } } },
        "traceId": { "type": "keyword", "ignore_above": 32 },
        "spanId": { "type": "keyword", "ignore_above": 16 },
        "parentSpanId": { "type": "keyword", "ignore_above": 16 },
        "name": { "ignore_above": 1024, "type": "keyword" },
        "traceState": { "ignore_above": 1024, "type": "keyword" },
        "traceGroup": { "ignore_above": 1024, "type": "keyword" },
        "traceGroupFields": { "properties": { "endTime": { "type": "date_nanos" }, "durationInNanos": { "type": "long" }, "statusCode": { "type": "integer" } } },
        "kind": { "type": "keyword", "ignore_above": 32 },
        "serviceName": { "type": "keyword", "ignore_above": 256 },
        "startTime": { "type": "date_nanos" },
        "endTime": { "type": "date_nanos" },
        "@timestamp": { "type": "date_nanos" },
        "time": { "type": "date_nanos" },
        "status": { "properties": { "code": { "type": "integer" }, "message": { "type": "keyword", "ignore_above": 2048 } } },
        "durationInNanos": { "type": "long" },
        "events": { "type": "nested", "properties": { "name": { "type": "keyword", "ignore_above": 256 }, "attributes": { "type": "object" }, "droppedAttributesCount": { "type": "integer" }, "time": { "type": "date_nanos" } } },
        "droppedEventsCount": { "type": "integer" },
        "links": { "type": "nested", "properties": { "traceId": { "type": "keyword", "ignore_above": 32 }, "spanId": { "type": "keyword", "ignore_above": 16 }, "traceState": { "ignore_above": 1024, "type": "keyword" }, "attributes": { "type": "object" }, "droppedAttributesCount": { "type": "integer" } } },
        "droppedLinksCount": { "type": "integer" }
      }
    }
  },
  "version": 1,
  "priority": 100
}`

	otelV1LogsTemplateBody = `{
  "index_patterns": ["otel-v1-logs*"],
  "template": {
    "mappings": {
      "date_detection": false,
      "_source": { "enabled": true },
      "dynamic_templates": [
        { "long_resource_attributes": { "mapping": { "type": "long" }, "path_match": "resource.attributes.*", "match_mapping_type": "long" } },
        { "double_resource_attributes": { "mapping": { "type": "double" }, "path_match": "resource.attributes.*", "match_mapping_type": "double" } },
        { "string_resource_attributes": { "mapping": { "type": "keyword", "ignore_above": 256 }, "path_match": "resource.attributes.*", "match_mapping_type": "string" } },
        { "long_scope_attributes": { "mapping": { "type": "long" }, "path_match": "instrumentationScope.attributes.*", "match_mapping_type": "long" } },
        { "double_scope_attributes": { "mapping": { "type": "double" }, "path_match": "instrumentationScope.attributes.*", "match_mapping_type": "double" } },
        { "string_scope_attributes": { "mapping": { "type": "keyword", "ignore_above": 256 }, "path_match": "instrumentationScope.attributes.*", "match_mapping_type": "string" } },
        { "long_attributes": { "mapping": { "type": "long" }, "path_match": "attributes.*", "match_mapping_type": "long" } },
        { "double_attributes": { "mapping": { "type": "double" }, "path_match": "attributes.*", "match_mapping_type": "double" } },
        { "string_attributes": { "mapping": { "type": "keyword", "ignore_above": 256 }, "path_match": "attributes.*", "match_mapping_type": "string" } }
      ],
      "properties": {
        "droppedAttributesCount": { "type": "integer" },
        "instrumentationScope": { "properties": { "droppedAttributesCount": { "type": "integer" }, "schemaUrl": { "type": "keyword", "ignore_above": 256 }, "name": { "type": "keyword", "ignore_above": 128 }, "version": { "type": "keyword", "ignore_above": 64 } } },
        "resource": { "properties": { "droppedAttributesCount": { "type": "integer" }, "schemaUrl": { "type": "keyword", "ignore_above": 256 } } },
        "severity": { "properties": { "number": { "type": "integer" }, "text": { "type": "keyword", "ignore_above": 32 } } },
        "body": { "type": "text" },
        "@timestamp": { "type": "date_nanos" },
        "time": { "type": "date_nanos" },
        "observedTime": { "type": "date_nanos" },
        "traceId": { "type": "keyword", "ignore_above": 32 },
        "spanId": { "type": "keyword", "ignore_above": 16 },
        "flags": { "type": "long" }
      }
    }
  },
  "version": 1,
  "priority": 100
}`
)

type templateManager struct {
	client *opensearchapi.Client
	logger *zap.Logger
}

func newTemplateManager(client *opensearchapi.Client, logger *zap.Logger) *templateManager {
	return &templateManager{client: client, logger: logger}
}

func (tm *templateManager) ensureTemplates(ctx context.Context) {
	tm.ensureTemplate(ctx, otelV1SpanTemplateName, otelV1SpanTemplateBody)
	tm.ensureTemplate(ctx, otelV1LogsTemplateName, otelV1LogsTemplateBody)
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
		tm.logger.Warn("Failed to create index template", zap.String("template", name), zap.Error(createErr))
		return
	}
	tm.logger.Info("Created index template", zap.String("template", name))
}
