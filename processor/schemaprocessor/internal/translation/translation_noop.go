// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translation

import (
	"context"

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

var (
	_ Translation = (*nopTranslation)(nil)
)

func (nopTranslation) SupportedVersion(_ *Version) bool {
	return false
}
func (nopTranslation) ApplyAllResourceChanges(_ context.Context, in alias.Resource)       {}
func (nopTranslation) ApplyScopeSpanChanges(_ context.Context, in ptrace.ScopeSpans)      {}
func (nopTranslation) ApplyScopeLogChanges(_ context.Context, in plog.ScopeLogs)          {}
func (nopTranslation) ApplyScopeMetricChanges(_ context.Context, in pmetric.ScopeMetrics) {}
