// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogrumreceiver/internal/translator"

import (
	"net/http"

	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

func ToLogs(payload map[string]any, req *http.Request, reqBytes []byte) plog.Logs {
	results := plog.NewLogs()
	rl := results.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl(semconv.SchemaURL)
	parseRUMRequestIntoResource(rl.Resource(), payload, req, reqBytes)

	in := rl.ScopeLogs().AppendEmpty()
	in.Scope().SetName(InstrumentationScopeName)

	newLogRecord := in.LogRecords().AppendEmpty()

	flatPayload := flattenJSON(payload)

	setAttributes(flatPayload, newLogRecord.Attributes())

	return results
}
