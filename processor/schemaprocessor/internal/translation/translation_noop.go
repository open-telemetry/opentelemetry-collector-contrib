// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
)

// NopTranslation defines a translation that performs no action
// or would force the processing path of no action.
// Used to be able to reduce the need of branching and
// keeping the same logic path
type nopTranslation struct{}

var _ Translation = (*nopTranslation)(nil)

func (nopTranslation) SupportedVersion(_ *Version) bool {
	return false
}

func (nopTranslation) ApplyAllResourceChanges(_ alias.Resource, _ string) error {
	return nil
}

func (nopTranslation) ApplyScopeSpanChanges(_ ptrace.ScopeSpans, _ string) error {
	return nil
}

func (nopTranslation) ApplyScopeLogChanges(_ plog.ScopeLogs, _ string) error {
	return nil
}

func (nopTranslation) ApplyScopeMetricChanges(_ pmetric.ScopeMetrics, _ string) error {
	return nil
}
