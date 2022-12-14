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

package ottlscope // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ottlcommon"
)

var _ ottlcommon.ResourceContext = TransformContext{}
var _ ottlcommon.InstrumentationScopeContext = TransformContext{}

type TransformContext struct {
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
	storage              pcommon.Map
}

func NewTransformContext(instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource) TransformContext {
	return TransformContext{
		instrumentationScope: instrumentationScope,
		resource:             resource,
		storage:              pcommon.NewMap(),
	}
}

func (tCtx TransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return tCtx.instrumentationScope
}

func (tCtx TransformContext) GetResource() pcommon.Resource {
	return tCtx.resource
}

func (tCtx TransformContext) getStorage() pcommon.Map {
	return tCtx.storage
}

func NewParser(functions map[string]interface{}, telemetrySettings component.TelemetrySettings) ottl.Parser[TransformContext] {
	return ottl.NewParser[TransformContext](functions, parsePath, parseEnum, telemetrySettings)
}

func parseEnum(val *ottl.EnumSymbol) (*ottl.Enum, error) {
	return nil, fmt.Errorf("instrumentation scope context does not provide Enum support")
}

func parsePath(val *ottl.Path) (ottl.GetSetter[TransformContext], error) {
	if val != nil && len(val.Fields) > 0 {
		return newPathGetSetter(val.Fields)
	}
	return nil, fmt.Errorf("bad path %v", val)
}

func newPathGetSetter(path []ottl.Field) (ottl.GetSetter[TransformContext], error) {
	switch path[0].Name {
	case "tmp":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessStorage(), nil
		}
		return accessStorageKey(mapKey), nil
	case "resource":
		return ottlcommon.ResourcePathGetSetter[TransformContext](path[1:])
	default:
		return ottlcommon.ScopePathGetSetter[TransformContext](path)
	}
}

func accessStorage() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx context.Context, tCtx TransformContext) (interface{}, error) {
			return tCtx.getStorage(), nil
		},
		Setter: func(ctx context.Context, tCtx TransformContext, val interface{}) error {
			if m, ok := val.(pcommon.Map); ok {
				m.CopyTo(tCtx.getStorage())
			}
			return nil
		},
	}
}

func accessStorageKey(mapKey *string) ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx context.Context, tCtx TransformContext) (interface{}, error) {
			return ottlcommon.GetMapValue(tCtx.getStorage(), *mapKey), nil
		},
		Setter: func(ctx context.Context, tCtx TransformContext, val interface{}) error {
			ottlcommon.SetMapValue(tCtx.getStorage(), *mapKey, val)
			return nil
		},
	}
}
