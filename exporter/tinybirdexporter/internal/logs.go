// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter/internal"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type logSignal struct {
	ResourceSchemaURL  string            `json:"resource_schema_url"`
	ResourceAttributes map[string]string `json:"resource_attributes"`
	ServiceName        string            `json:"service_name"`
	ScopeSchemaURL     string            `json:"scope_schema_url"`
	ScopeAttributes    map[string]string `json:"scope_attributes"`
	ScopeName          string            `json:"scope_name"`
	ScopeVersion       string            `json:"scope_version"`
	Timestamp          string            `json:"timestamp"`
	TraceID            string            `json:"trace_id"`
	SpanID             string            `json:"span_id"`
	Flags              uint32            `json:"flags"`
	SeverityText       string            `json:"severity_text"`
	SeverityNumber     int32             `json:"severity_number"`
	LogAttributes      map[string]string `json:"log_attributes"`
	Body               string            `json:"body"`
}

func ConvertLogs(ld plog.Logs, encoder Encoder) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resourceSchemaURL := rl.SchemaUrl()
		resource := rl.Resource()
		resourceAttributesMap := resource.Attributes()
		resourceAttributes := convertAttributes(resourceAttributesMap)
		serviceName := getServiceName(resource.Attributes())
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			scopeSchemaURL := sl.SchemaUrl()
			scope := sl.Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()
			scopeAttributes := convertAttributes(scope.Attributes())
			for k := 0; k < sl.LogRecords().Len(); k++ {
				log := sl.LogRecords().At(k)

				timestamp := log.Timestamp()
				if timestamp == 0 {
					timestamp = log.ObservedTimestamp()
				}

				logEntry := logSignal{
					ResourceSchemaURL:  resourceSchemaURL,
					ResourceAttributes: resourceAttributes,
					ServiceName:        serviceName,
					ScopeName:          scopeName,
					ScopeVersion:       scopeVersion,
					ScopeSchemaURL:     scopeSchemaURL,
					ScopeAttributes:    scopeAttributes,
					Timestamp:          timestamp.AsTime().Format(time.RFC3339Nano),
					SeverityText:       log.SeverityText(),
					SeverityNumber:     int32(log.SeverityNumber()),
					LogAttributes:      convertAttributes(log.Attributes()),
					Body:               log.Body().AsString(),
					TraceID:            traceutil.TraceIDToHexOrEmptyString(log.TraceID()),
					SpanID:             traceutil.SpanIDToHexOrEmptyString(log.SpanID()),
					Flags:              uint32(log.Flags()),
				}

				err := encoder.Encode(logEntry)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
