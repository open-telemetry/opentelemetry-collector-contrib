// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"encoding/hex"
	"testing"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

var emptyScopeSpansAndEmptyResourceAttributes = func() ptrace.Traces {
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty()
	return td
}

var twoResourceSpansWithDifferentServiceNameResourceAttribute = func(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rss1 := td.ResourceSpans()
	rs1 := rss1.AppendEmpty()
	rs1.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "testapp")
	scopeSpans1 := generateScopeSpans(t)
	scopeSpans1.CopyTo(rs1.ScopeSpans().AppendEmpty())

	rss2 := td.ResourceSpans()
	rs2 := rss2.AppendEmpty()
	rs2.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "testapp-second")
	scopeSpans2 := generateScopeSpans(t)
	scopeSpans2.CopyTo(rs2.ScopeSpans().AppendEmpty())

	return td
}

var twoResourceSpansWithTheSameResource = func(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rss := td.ResourceSpans()
	rs1 := rss.AppendEmpty()
	rs1.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "testapp")
	scopeSpans1 := generateScopeSpans(t)
	scopeSpans1.CopyTo(rs1.ScopeSpans().AppendEmpty())
	rs2 := rss.AppendEmpty()
	rs2.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "testapp")
	scopeSpans2 := generateScopeSpans(t)
	scopeSpans2.CopyTo(rs2.ScopeSpans().AppendEmpty())

	return td
}

func generateScopeSpans(t *testing.T) ptrace.ScopeSpans {
	t.Helper()
	scopeSpans := ptrace.NewScopeSpans()
	scope := scopeSpans.Scope()
	scope.SetName("@opentelemetry/instrumentation-fetch")
	scope.SetVersion("0.48.0")
	spans := scopeSpans.Spans()

	span := spans.AppendEmpty()
	span.SetName("HTTP GET")
	span.SetKind(3)
	span.SetStartTimestamp(pcommon.Timestamp(1718700770771000000))
	span.SetEndTimestamp(pcommon.Timestamp(1718700770800000000))
	spanID, err := generateSpanIDFromString("f71b2cc42962650f")
	require.NoError(t, err)
	span.SetSpanID(spanID)
	traceID, err := generateTraceIDFromString("bac44bf5d6d040fc5fdf9bc22442c6f2")
	require.NoError(t, err)
	span.SetTraceID(traceID)

	return scopeSpans
}

var generateSpanIDFromString = func(s string) ([8]byte, error) {
	var spanID [8]byte
	_, err := hex.Decode(spanID[:], []byte(s))
	if err != nil {
		return spanID, err
	}
	return spanID, nil
}

var generateTraceIDFromString = func(s string) ([16]byte, error) {
	var traceID [16]byte
	_, err := hex.Decode(traceID[:], []byte(s))
	if err != nil {
		return traceID, err
	}
	return traceID, nil
}

func TestTranslateFromTraces(t *testing.T) {
	testcases := []struct {
		name         string
		td           ptrace.Traces
		wantPayloads []*faroTypes.Payload
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			name: "Empty traces",
			td:   ptrace.NewTraces(),
			wantPayloads: func() []*faroTypes.Payload {
				return nil
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "Empty scope spans and empty resource attribute",
			td:   emptyScopeSpansAndEmptyResourceAttributes(),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "Two resource spans with different resource attributes should produce two faro payloads",
			td:   twoResourceSpansWithDifferentServiceNameResourceAttribute(t),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "payload-traces-meta-app-name-1.json"))
				payloads = append(payloads, PayloadFromFile(t, "payload-traces-meta-app-name-2.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "Two resource spans with the same resource attributes should produce one faro payload",
			td:   twoResourceSpansWithTheSameResource(t),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "payload-traces-two-traces-same-resource.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			payloads, err := TranslateFromTraces(context.TODO(), tt.td)
			tt.wantErr(t, err)
			assert.ElementsMatch(t, tt.wantPayloads, payloads)
		})
	}
}

func Test_extractMetaFromResourceAttributes(t *testing.T) {
	testcases := []struct {
		name               string
		resourceAttributes pcommon.Map
		wantMeta           faroTypes.Meta
	}{
		{
			name: "Resource attributes contain all the attributes for meta",
			resourceAttributes: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr(string(semconv.ServiceNameKey), "testapp")
				attrs.PutStr(string(semconv.ServiceNamespaceKey), "testnamespace")
				attrs.PutStr(string(semconv.ServiceVersionKey), "1.0.0")
				attrs.PutStr(string(semconv.DeploymentEnvironmentKey), "production")
				attrs.PutStr("app_bundle_id", "123")
				attrs.PutStr(string(semconv.TelemetrySDKNameKey), "telemetry sdk")
				attrs.PutStr(string(semconv.TelemetrySDKVersionKey), "1.0.0")

				return attrs
			}(),
			wantMeta: faroTypes.Meta{
				App: faroTypes.App{
					Name:        "testapp",
					Namespace:   "testnamespace",
					Version:     "1.0.0",
					Environment: "production",
					BundleID:    "123",
				},
				SDK: faroTypes.SDK{
					Name:    "telemetry sdk",
					Version: "1.0.0",
				},
			},
		},
		{
			name:               "Resource attributes don't contain attributes for meta",
			resourceAttributes: pcommon.NewMap(),
			wantMeta:           faroTypes.Meta{},
		},
	}
	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.wantMeta, extractMetaFromResourceAttributes(tt.resourceAttributes), "extractMetaFromResourceAttributes(%v)", tt.resourceAttributes.AsRaw())
		})
	}
}
