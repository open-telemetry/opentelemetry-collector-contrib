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

func ResourcePathGetSetter(path []ottl.Field) (ottl.GetSetter, error) {
	if len(path) == 0 {
		return accessResource(), nil
	}
	switch path[0].Name {
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessResourceAttributes(), nil
		}
		return accessResourceAttributesKey(mapKey), nil
	case "dropped_attributes_count":
		return accessDroppedAttributesCount(), nil
	}

	return nil, fmt.Errorf("invalid resource path expression %v", path)
}

func accessResource() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetResource()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if newRes, ok := val.(pcommon.Resource); ok {
				newRes.CopyTo(ctx.GetResource())
			}
		},
	}
}

func accessResourceAttributes() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetResource().Attributes()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				attrs.CopyTo(ctx.GetResource().Attributes())
			}
		},
	}
}

func accessResourceAttributesKey(mapKey *string) ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return GetMapValue(ctx.GetResource().Attributes(), *mapKey)
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			SetMapValue(ctx.GetResource().Attributes(), *mapKey, val)
		},
	}
}

func accessDroppedAttributesCount() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return int64(ctx.GetResource().DroppedAttributesCount())
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetResource().SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}
