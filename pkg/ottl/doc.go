// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package ottl implements the OpenTelemetry Transformation Language.
//
// Observability
//
// OTTL's internal telemetry is limited to structured logs emitted via
// [go.opentelemetry.io/collector/component.TelemetrySettings]. The log
// contract is Stable and will not change in a backward-incompatible way
// without a major version bump:
//
//   - Debug level ([go.uber.org/zap.DebugLevel]): highly verbose, gated by
//     [go.uber.org/zap.Core.Enabled]. It emits the statement/condition text
//     and the full TransformContext (resource, scope, and signal data) before
//     and after execution. This is Detailed telemetry per
//     go.opentelemetry.io/collector/config/configtelemetry and MUST NOT be
//     enabled in production without considering that it contains raw signal data
//     (attributes, bodies, span names, etc.) and otelcol.* client metadata when
//     ottl.contexts.enableOTelColContext is enabled.
//
//   - Warn level ([go.uber.org/zap.WarnLevel]): emitted only when
//     [ErrorMode] is [IgnoreError] and a statement or condition returns an
//     error. It contains only the statement/condition text and the error, never
//     signal data, so it is safe for Normal telemetry and suitable for alerting
//     via log scraping. When [ErrorMode] is [PropagateError] the error is
//     returned to the caller and observed via the host pipeline component's
//     otelcol.*.consumed/produced metrics; when Silent, no log is emitted.
//
// Host pipeline components (transformprocessor, filterprocessor, routingconnector,
// tailsamplingprocessor) provide the pipeline-level observability required by
// go.opentelemetry.io/collector/docs/component-stability.md#stable
// (received/output items, dropped counts, queue depth, and latency histograms)
// via their own mdatagen telemetry. OTTL itself does not emit metrics or traces
// so its API remains lightweight and its telemetry does not duplicate the host's
// component-instance attributes.
package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
