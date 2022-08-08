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

package testhelper // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func Strp(s string) *string {
	return &s
}

func Floatp(f float64) *float64 {
	return &f
}

func Intp(i int64) *int64 {
	return &i
}

func Boolp(b bool) *bool {
	return &b
}

type TestTransformContext struct {
	Item interface{}
}

func (ctx TestTransformContext) GetItem() interface{} {
	return ctx.Item
}

func (ctx TestTransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return pcommon.InstrumentationScope{}
}

func (ctx TestTransformContext) GetResource() pcommon.Resource {
	return pcommon.Resource{}
}
