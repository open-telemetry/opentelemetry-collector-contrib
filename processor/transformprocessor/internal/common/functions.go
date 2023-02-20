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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func Functions[K any]() map[string]interface{} {
	return map[string]interface{}{
		"TraceID":              ottlfuncs.TraceID[K],
		"SpanID":               ottlfuncs.SpanID[K],
		"IsMatch":              ottlfuncs.IsMatch[K],
		"Concat":               ottlfuncs.Concat[K],
		"Split":                ottlfuncs.Split[K],
		"Int":                  ottlfuncs.Int[K],
		"ConvertCase":          ottlfuncs.ConvertCase[K],
		"ParseJSON":            ottlfuncs.ParseJSON[K],
		"Substring":            ottlfuncs.Substring[K],
		"keep_keys":            ottlfuncs.KeepKeys[K],
		"set":                  ottlfuncs.Set[K],
		"truncate_all":         ottlfuncs.TruncateAll[K],
		"limit":                ottlfuncs.Limit[K],
		"replace_match":        ottlfuncs.ReplaceMatch[K],
		"replace_all_matches":  ottlfuncs.ReplaceAllMatches[K],
		"replace_pattern":      ottlfuncs.ReplacePattern[K],
		"replace_all_patterns": ottlfuncs.ReplaceAllPatterns[K],
		"delete_key":           ottlfuncs.DeleteKey[K],
		"delete_matching_keys": ottlfuncs.DeleteMatchingKeys[K],
		"merge_maps":           ottlfuncs.MergeMaps[K],
	}
}

func ResourceFunctions() map[string]interface{} {
	return Functions[ottlresource.TransformContext]()
}

func ScopeFunctions() map[string]interface{} {
	return Functions[ottlscope.TransformContext]()
}
