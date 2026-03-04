// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package faro provides utilities for bidirectional translation between Grafana Faro
// and OpenTelemetry (OTLP) formats. These translation utilities are used by both
// the Faro receiver and Faro exporter components to ensure seamless data flow
// between Faro and OpenTelemetry systems.
package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	faroTypes "github.com/grafana/faro/pkg/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	conventionsv126 "go.opentelemetry.io/otel/semconv/v1.26.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/multierr"
)

// TranslateFromTraces converts a Traces pipeline data into []*faro.Payload
func TranslateFromTraces(ctx context.Context, td ptrace.Traces) ([]faroTypes.Payload, error) {
	_, span := otel.Tracer("").Start(ctx, "TranslateFromTraces")
	defer span.End()

	resourceSpans := td.ResourceSpans()
	resourceSpansLen := resourceSpans.Len()
	if resourceSpansLen == 0 {
		return nil, nil
	}
	metaMap := make(map[string]*faroTypes.Payload)
	var payloads []faroTypes.Payload
	w := sha256.New()
	encoder := json.NewEncoder(w)
	var errs error
	for i := range resourceSpansLen {
		rs := resourceSpans.At(i)
		payload := resourceSpansToFaroPayload(rs)
		meta := payload.Meta
		w.Reset()
		if encodeErr := encoder.Encode(meta); encodeErr != nil {
			errs = multierr.Append(errs, encodeErr)
			continue
		}
		metaKey := fmt.Sprintf("%x", w.Sum(nil))
		// if payload meta already exists in the metaMap merge payload to the existing payload
		existingPayload, found := metaMap[metaKey]
		if found {
			mergePayloads(existingPayload, payload)
		} else {
			metaMap[metaKey] = &payload
		}
	}
	if len(metaMap) == 0 {
		return payloads, errs
	}
	payloads = make([]faroTypes.Payload, 0)
	for _, payload := range metaMap {
		payloads = append(payloads, *payload)
	}
	span.SetAttributes(attribute.Int("count", len(payloads)))
	return payloads, errs
}

func resourceSpansToFaroPayload(rs ptrace.ResourceSpans) faroTypes.Payload {
	var payload faroTypes.Payload

	resource := rs.Resource()
	scopeSpans := rs.ScopeSpans()

	if resource.Attributes().Len() == 0 && scopeSpans.Len() == 0 {
		return payload
	}

	traces := ptrace.NewTraces()
	rs.CopyTo(traces.ResourceSpans().AppendEmpty())

	meta := extractMetaFromResourceAttributes(resource.Attributes())
	payload = faroTypes.Payload{
		Meta: meta,
		Traces: &faroTypes.Traces{
			Traces: traces,
		},
	}

	return payload
}

func extractMetaFromResourceAttributes(resourceAttributes pcommon.Map) faroTypes.Meta {
	var meta faroTypes.Meta
	var app faroTypes.App
	var sdk faroTypes.SDK

	if appName, ok := resourceAttributes.Get(string(conventions.ServiceNameKey)); ok {
		app.Name = appName.Str()
	}
	if appNamespace, ok := resourceAttributes.Get(string(conventions.ServiceNamespaceKey)); ok {
		app.Namespace = appNamespace.Str()
	}
	if version, ok := resourceAttributes.Get(string(conventions.ServiceVersionKey)); ok {
		app.Version = version.Str()
	}
	if environment, ok := resourceAttributes.Get(string(conventionsv126.DeploymentEnvironmentKey)); ok {
		app.Environment = environment.Str()
	}
	if appBundleID, ok := resourceAttributes.Get(faroAppBundleID); ok {
		app.BundleID = appBundleID.Str()
	}
	if sdkName, ok := resourceAttributes.Get(string(conventions.TelemetrySDKNameKey)); ok {
		sdk.Name = sdkName.Str()
	}
	if sdkVersion, ok := resourceAttributes.Get(string(conventions.TelemetrySDKVersionKey)); ok {
		sdk.Version = sdkVersion.Str()
	}

	meta.App = app
	meta.SDK = sdk
	return meta
}
