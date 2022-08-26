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

package tqlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/internal/tqlcommon"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

func ScopePathGetSetter(path []tql.Field) (tql.GetSetter, error) {
	if len(path) == 0 {
		return accessInstrumentationScope(), nil
	}

	switch path[0].Name {
	case "name":
		return accessInstrumentationScopeName(), nil
	case "version":
		return accessInstrumentationScopeVersion(), nil
	}

	return nil, fmt.Errorf("invalid scope path expression %v", path)
}

func accessInstrumentationScope() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetInstrumentationScope()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newIl, ok := val.(pcommon.InstrumentationScope); ok {
				newIl.CopyTo(ctx.GetInstrumentationScope())
			}
		},
	}
}

func accessInstrumentationScopeName() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Name()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetName(str)
			}
		},
	}
}

func accessInstrumentationScopeVersion() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Version()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetVersion(str)
			}
		},
	}
}
