// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxspanlink // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspanlink"

import "go.opentelemetry.io/collector/pdata/ptrace"

const (
	Name   = "spanlink"
	DocRef = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlspanlink"
)

type Context interface {
	GetSpanLink() ptrace.SpanLink
}
