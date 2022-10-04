// Copyright The OpenTelemetry Authors
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

package ottlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ottlcommon"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type InstrumentationScopeContext interface {
	GetInstrumentationScope() pcommon.InstrumentationScope
}

func ScopePathGetSetter[K InstrumentationScopeContext](path []ottl.Field) (ottl.GetSetter[K], error) {
	if len(path) == 0 {
		return accessInstrumentationScope[K](), nil
	}

	switch path[0].Name {
	case "name":
		return accessInstrumentationScopeName[K](), nil
	case "version":
		return accessInstrumentationScopeVersion[K](), nil
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessInstrumentationScopeAttributes[K](), nil
		}
		return accessInstrumentationScopeAttributesKey[K](mapKey), nil
	}

	return nil, fmt.Errorf("invalid scope path expression %v", path)
}

func accessInstrumentationScope[K InstrumentationScopeContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetInstrumentationScope()
		},
		Setter: func(ctx K, val interface{}) {
			if newIl, ok := val.(pcommon.InstrumentationScope); ok {
				newIl.CopyTo(ctx.GetInstrumentationScope())
			}
		},
	}
}

func accessInstrumentationScopeAttributes[K InstrumentationScopeContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetInstrumentationScope().Attributes()
		},
		Setter: func(ctx K, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				attrs.CopyTo(ctx.GetInstrumentationScope().Attributes())
			}
		},
	}
}

func accessInstrumentationScopeAttributesKey[K InstrumentationScopeContext](mapKey *string) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return GetMapValue(ctx.GetInstrumentationScope().Attributes(), *mapKey)
		},
		Setter: func(ctx K, val interface{}) {
			SetMapValue(ctx.GetInstrumentationScope().Attributes(), *mapKey, val)
		},
	}
}

func accessInstrumentationScopeName[K InstrumentationScopeContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetInstrumentationScope().Name()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetName(str)
			}
		},
	}
}

func accessInstrumentationScopeVersion[K InstrumentationScopeContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetInstrumentationScope().Version()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetVersion(str)
			}
		},
	}
}
