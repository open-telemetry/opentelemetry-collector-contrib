// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogsemanticsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor"

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/DataDog/datadog-agent/pkg/trace/otel/traceutil"
	"github.com/DataDog/datadog-agent/pkg/trace/transform"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

func (tp *tracesProcessor) insertAttrIfMissingOrShouldOverride(sattr pcommon.Map, key string, value any) (err error) {
	if _, ok := sattr.Get(key); tp.overrideIncomingDatadogFields || !ok {
		switch v := value.(type) {
		case string:
			sattr.PutStr(key, v)
		case int64:
			sattr.PutInt(key, v)
		default:
			err = errors.New("unsupported value type")
		}
	}
	return err
}

func (tp *tracesProcessor) processTraces(ctx context.Context, td ptrace.Traces) (output ptrace.Traces, err error) {
	rspans := td.ResourceSpans()
	for i := 0; i < rspans.Len(); i++ {
		rspan := rspans.At(i)
		otelres := rspan.Resource()
		rattr := otelres.Attributes()
		for j := 0; j < rspan.ScopeSpans().Len(); j++ {
			serviceVersion := ""
			if serviceVersionAttr, ok := otelres.Attributes().Get(string(conventions.ServiceVersionKey)); ok {
				serviceVersion = serviceVersionAttr.AsString()
			}
			err = tp.insertAttrIfMissingOrShouldOverride(rattr, "datadog.version", serviceVersion)
			if err != nil {
				return ptrace.Traces{}, err
			}

			if tp.overrideIncomingDatadogFields {
				rattr.Remove("datadog.host.name")
			}

			datadogHostName := ""
			if src, ok := tp.attrsTranslator.ResourceToSource(ctx, otelres, traceutil.SignalTypeSet, nil); ok && src.Kind == source.HostnameKind {
				datadogHostName = src.Identifier
			}
			err = tp.insertAttrIfMissingOrShouldOverride(rattr, "datadog.host.name", datadogHostName)
			if err != nil {
				return ptrace.Traces{}, err
			}

			// Map VCS (version control system) attributes for source code integration at resource level
			if vcsRevision, ok := rattr.Get(string(conventions.VCSRefHeadRevisionKey)); ok {
				err = tp.insertAttrIfMissingOrShouldOverride(rattr, "git.commit.sha", vcsRevision.AsString())
				if err != nil {
					return ptrace.Traces{}, err
				}
			}
			if vcsRepoURL, ok := rattr.Get(string(conventions.VCSRepositoryURLFullKey)); ok {
				// Strip protocol from repository URL as required by Datadog
				cleanURL := stripProtocolFromURL(vcsRepoURL.AsString())
				err = tp.insertAttrIfMissingOrShouldOverride(rattr, "git.repository_url", cleanURL)
				if err != nil {
					return ptrace.Traces{}, err
				}
			}

			libspans := rspan.ScopeSpans().At(j)
			for k := 0; k < libspans.Spans().Len(); k++ {
				otelspan := libspans.Spans().At(k)
				sattr := otelspan.Attributes()

				// Note: default value from GetOTelService is "otlpresourcenoservicename"
				err = tp.insertAttrIfMissingOrShouldOverride(rattr, "datadog.service", traceutil.GetOTelService(otelspan, otelres, true))
				if err != nil {
					return ptrace.Traces{}, err
				}

				env := "default"
				if envFromAttr := transform.GetOTelEnv(otelspan, otelres); envFromAttr != "" {
					env = envFromAttr
				}
				err = tp.insertAttrIfMissingOrShouldOverride(rattr, "datadog.env", env)
				if err != nil {
					return ptrace.Traces{}, err
				}

				err = tp.insertAttrIfMissingOrShouldOverride(sattr, "datadog.name", traceutil.GetOTelOperationNameV2(otelspan, otelres))
				if err != nil {
					return ptrace.Traces{}, err
				}
				err = tp.insertAttrIfMissingOrShouldOverride(sattr, "datadog.resource", traceutil.GetOTelResourceV2(otelspan, otelres))
				if err != nil {
					return ptrace.Traces{}, err
				}
				err = tp.insertAttrIfMissingOrShouldOverride(sattr, "datadog.type", traceutil.GetOTelSpanType(otelspan, otelres))
				if err != nil {
					return ptrace.Traces{}, err
				}
				spanKind := otelspan.Kind()
				err = tp.insertAttrIfMissingOrShouldOverride(sattr, "datadog.span.kind", traceutil.OTelSpanKindName(spanKind))
				if err != nil {
					return ptrace.Traces{}, err
				}

				// Map VCS (version control system) attributes for source code integration
				if vcsRevision, ok := sattr.Get(string(conventions.VCSRefHeadRevisionKey)); ok {
					err = tp.insertAttrIfMissingOrShouldOverride(sattr, "git.commit.sha", vcsRevision.AsString())
					if err != nil {
						return ptrace.Traces{}, err
					}
				}
				if vcsRepoURL, ok := sattr.Get(string(conventions.VCSRepositoryURLFullKey)); ok {
					// Strip protocol from repository URL as required by Datadog
					cleanURL := stripProtocolFromURL(vcsRepoURL.AsString())
					err = tp.insertAttrIfMissingOrShouldOverride(sattr, "git.repository_url", cleanURL)
					if err != nil {
						return ptrace.Traces{}, err
					}
				}

				metaMap := make(map[string]string)
				code := transform.GetOTelStatusCode(otelspan, otelres)
				if code != 0 {
					err = tp.insertAttrIfMissingOrShouldOverride(sattr, "datadog.http_status_code", fmt.Sprintf("%d", code))
					if err != nil {
						return ptrace.Traces{}, err
					}
				}
				ddError := int64(status2Error(otelspan.Status(), otelspan.Events(), metaMap))
				err = tp.insertAttrIfMissingOrShouldOverride(sattr, "datadog.error", ddError)
				if err != nil {
					return ptrace.Traces{}, err
				}
				if ddError == 1 {
					err = tp.insertAttrIfMissingOrShouldOverride(sattr, "datadog.error.msg", metaMap["error.msg"])
					if err != nil {
						return ptrace.Traces{}, err
					}
					err = tp.insertAttrIfMissingOrShouldOverride(sattr, "datadog.error.type", metaMap["error.type"])
					if err != nil {
						return ptrace.Traces{}, err
					}
					err = tp.insertAttrIfMissingOrShouldOverride(sattr, "datadog.error.stack", metaMap["error.stack"])
					if err != nil {
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
		if v, ok := attrs.Get(string(conventions.ExceptionMessageKey)); ok {
			metaMap["error.msg"] = v.AsString()
		}
		if v, ok := attrs.Get(string(conventions.ExceptionTypeKey)); ok {
			metaMap["error.type"] = v.AsString()
		}
		if v, ok := attrs.Get(string(conventions.ExceptionStacktraceKey)); ok {
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

func stripProtocolFromURL(rawURL string) string {
	// Try to parse as URL first
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, return the original string
		return rawURL
	}

	return strings.TrimPrefix(rawURL, parsedURL.Scheme+"://")
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
