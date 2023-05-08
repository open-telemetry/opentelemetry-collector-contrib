// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package converter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

type Converter interface {
	AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool
	ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle
	Name() string
}
