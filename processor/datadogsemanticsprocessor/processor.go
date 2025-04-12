// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogsemanticsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-agent/pkg/trace/traceutil"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
)

func insertAttrIfMissing(sattr pcommon.Map, key string, value any) (err error) {
	if _, ok := sattr.Get(key); !ok {
		switch v := value.(type) {
		case string:
			sattr.PutStr(key, v)
		case int64:
			sattr.PutInt(key, v)
		default:
			err = errors.New("unsupported value type")
		}
	}
	return
}

func (tp *tracesProcessor) processTraces(ctx context.Context, td ptrace.Traces) (output ptrace.Traces, err error) {
	rspans := td.ResourceSpans()
	for i := 0; i < rspans.Len(); i++ {
		rspan := rspans.At(i)
		otelres := rspan.Resource()
		rattr := otelres.Attributes()
		for j := 0; j < rspan.ScopeSpans().Len(); j++ {
			libspans := rspan.ScopeSpans().At(j)
			for k := 0; k < libspans.Spans().Len(); k++ {
				otelspan := libspans.Spans().At(k)
				sattr := otelspan.Attributes()
				if tp.overrideIncomingDatadogFields {
					sattr.RemoveIf(func(k string, _ pcommon.Value) bool {
						return strings.HasPrefix(k, "datadog.")
					})
					if ddHostname, ok := rattr.Get("datadog.host.name"); ok && ddHostname.AsString() == "" {
						rattr.Remove("datadog.host.name")
					}
				}
				if err = insertAttrIfMissing(sattr, "datadog.service", traceutil.GetOTelService(otelres, true)); err != nil {
					return ptrace.Traces{}, err
				}
				if err = insertAttrIfMissing(sattr, "datadog.name", traceutil.GetOTelOperationNameV2(otelspan)); err != nil {
					return ptrace.Traces{}, err
				}
				if err = insertAttrIfMissing(sattr, "datadog.resource", traceutil.GetOTelResourceV2(otelspan, otelres)); err != nil {
					return ptrace.Traces{}, err
				}
				if err = insertAttrIfMissing(sattr, "datadog.type", traceutil.GetOTelSpanType(otelspan, otelres)); err != nil {
					return ptrace.Traces{}, err
				}
				if src, ok := tp.attrsTranslator.ResourceToSource(ctx, otelres, traceutil.SignalTypeSet, nil); ok && src.Kind == source.HostnameKind {
					if err = insertAttrIfMissing(otelres.Attributes(), "datadog.host.name", src.Identifier); err != nil {
						return ptrace.Traces{}, err
					}
				}

				spanKind := otelspan.Kind()
				if err = insertAttrIfMissing(sattr, "datadog.span.kind", traceutil.OTelSpanKindName(spanKind)); err != nil {
					return ptrace.Traces{}, err
				}
				if env := traceutil.GetOTelEnv(otelres); env != "" {
					if err = insertAttrIfMissing(sattr, "datadog.env", env); err != nil {
						return ptrace.Traces{}, err
					}
				}
				if serviceVersion, ok := otelres.Attributes().Get(semconv.AttributeServiceVersion); ok {
					if err = insertAttrIfMissing(sattr, "datadog.version", serviceVersion.AsString()); err != nil {
						return ptrace.Traces{}, err
					}
				}

				metaMap := make(map[string]string)
				code := traceutil.GetOTelStatusCode(otelspan)
				if code != 0 {
					if err = insertAttrIfMissing(sattr, "datadog.http_status_code", fmt.Sprintf("%d", code)); err != nil {
						return ptrace.Traces{}, err
					}
				}
				ddError := int64(status2Error(otelspan.Status(), otelspan.Events(), metaMap))
				if err = insertAttrIfMissing(sattr, "datadog.error", ddError); err != nil {
					return ptrace.Traces{}, err
				}
				if metaMap["error.msg"] != "" {
					if err = insertAttrIfMissing(sattr, "datadog.error.msg", metaMap["error.msg"]); err != nil {
						return ptrace.Traces{}, err
					}
				}
				if metaMap["error.type"] != "" {
					if err = insertAttrIfMissing(sattr, "datadog.error.type", metaMap["error.type"]); err != nil {
						return ptrace.Traces{}, err
					}
				}
				if metaMap["error.stack"] != "" {
					if err = insertAttrIfMissing(sattr, "datadog.error.stack", metaMap["error.stack"]); err != nil {
						return ptrace.Traces{}, err
					}
				}
			}
		}
	}

	return td, err
}

// TODO import this from datadog-agent pending https://github.com/DataDog/datadog-agent/pull/33753
// Status2Error...
func status2Error(status ptrace.Status, events ptrace.SpanEventSlice, metaMap map[string]string) int32 {
	if status.Code() != ptrace.StatusCodeError {
		return 0
	}
	for i := 0; i < events.Len(); i++ {
		e := events.At(i)
		if strings.ToLower(e.Name()) != "exception" {
			continue
		}
		attrs := e.Attributes()
		if v, ok := attrs.Get(semconv.AttributeExceptionMessage); ok {
			metaMap["error.msg"] = v.AsString()
		}
		if v, ok := attrs.Get(semconv.AttributeExceptionType); ok {
			metaMap["error.type"] = v.AsString()
		}
		if v, ok := attrs.Get(semconv.AttributeExceptionStacktrace); ok {
			metaMap["error.stack"] = v.AsString()
		}
	}
	if _, ok := metaMap["error.msg"]; !ok {
		// no error message was extracted, find alternatives
		if status.Message() != "" {
			// use the status message
			metaMap["error.msg"] = status.Message()
		} else if _, httpcode := getFirstFromMap(metaMap, "http.response.status_code", "http.status_code"); httpcode != "" {
			// `http.status_code` was renamed to `http.response.status_code` in the HTTP stabilization from v1.23.
			// See https://opentelemetry.io/docs/specs/semconv/http/migration-guide/#summary-of-changes

			// http.status_text was removed in spec v0.7.0 (https://github.com/open-telemetry/opentelemetry-specification/pull/972)
			// TODO (OTEL-1791) Remove this and use a map from status code to status text.
			if httptext, ok := metaMap["http.status_text"]; ok {
				metaMap["error.msg"] = fmt.Sprintf("%s %s", httpcode, httptext)
			} else {
				metaMap["error.msg"] = httpcode
			}
		}
	}
	return 1
}

// TODO remove once Status2Error is imported from datadog-agent
func getFirstFromMap(m map[string]string, keys ...string) (string, string) {
	for _, key := range keys {
		if val := m[key]; val != "" {
			return key, val
		}
	}
	return "", ""
}
