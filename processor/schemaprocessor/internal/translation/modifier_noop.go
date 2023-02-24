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

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
)

// NopModifier is used when there is no definition present for a given change
type NopModifier struct{}

var _ Modifier = (*NopModifier)(nil)

func (NopModifier) UpdateAttrs(attrs pcommon.Map)                 {}
func (NopModifier) RevertAttrs(attrs pcommon.Map)                 {}
func (NopModifier) UpdateAttrsIf(match string, attrs pcommon.Map) {}
func (NopModifier) RevertAttrsIf(match string, attrs pcommon.Map) {}
func (NopModifier) UpdateSignal(signal alias.Signal)              {}
func (NopModifier) RevertSignal(signal alias.Signal)              {}
