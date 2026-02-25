// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"

	faroTypes "github.com/grafana/faro/pkg/go"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	conventionsv126 "go.opentelemetry.io/otel/semconv/v1.26.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

// TranslateToTraces converts faro.Payload into Traces pipeline data
func TranslateToTraces(ctx context.Context, payload faroTypes.Payload) (ptrace.Traces, error) {
	_, span := otel.Tracer("").Start(ctx, "TranslateToTraces")
	defer span.End()

	traces := ptrace.NewTraces()
	if payload.Traces == nil {
		return traces, nil
	}

	resspanCount := payload.Traces.Traces.ResourceSpans().Len()
	span.SetAttributes(attribute.Int("count", resspanCount))
	traces.ResourceSpans().EnsureCapacity(resspanCount)
	for i := range resspanCount {
		rs := traces.ResourceSpans().AppendEmpty()
		frs := ptrace.NewResourceSpans()
		payload.Traces.Traces.ResourceSpans().At(i).CopyTo(frs)
		frs.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), payload.Meta.App.Name)
		frs.Resource().Attributes().PutStr(string(conventions.ServiceVersionKey), payload.Meta.App.Version)
		frs.Resource().Attributes().PutStr(string(conventionsv126.DeploymentEnvironmentKey), payload.Meta.App.Environment)

		if payload.Meta.App.Namespace != "" {
			frs.Resource().Attributes().PutStr(string(conventions.ServiceNamespaceKey), payload.Meta.App.Namespace)
		}
		frs.CopyTo(rs)
	}

	return traces, nil
}
