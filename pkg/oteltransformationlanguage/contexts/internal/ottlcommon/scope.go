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

package ottlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/contexts/internal/ottlcommon"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottl"
)

func ScopePathGetSetter(path []ottl.Field) (ottl.GetSetter, error) {
	if len(path) == 0 {
		return accessInstrumentationScope(), nil
	}

	switch path[0].Name {
	case "name":
		return accessInstrumentationScopeName(), nil
	case "version":
		return accessInstrumentationScopeVersion(), nil
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessInstrumentationScopeAttributes(), nil
		}
		return accessInstrumentationScopeAttributesKey(mapKey), nil
	}

	return nil, fmt.Errorf("invalid scope path expression %v", path)
}

func accessInstrumentationScope() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetInstrumentationScope()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if newIl, ok := val.(pcommon.InstrumentationScope); ok {
				newIl.CopyTo(ctx.GetInstrumentationScope())
			}
		},
	}
}

func accessInstrumentationScopeAttributes() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Attributes()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				attrs.CopyTo(ctx.GetInstrumentationScope().Attributes())
			}
		},
	}
}

func accessInstrumentationScopeAttributesKey(mapKey *string) ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return GetMapValue(ctx.GetInstrumentationScope().Attributes(), *mapKey)
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			SetMapValue(ctx.GetInstrumentationScope().Attributes(), *mapKey, val)
		},
	}
}

func accessInstrumentationScopeName() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Name()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetName(str)
			}
		},
	}
}

func accessInstrumentationScopeVersion() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Version()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetVersion(str)
			}
		},
	}
}
