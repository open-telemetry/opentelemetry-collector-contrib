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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

var registry = map[string]interface{}{
	"TraceID":              ottlfuncs.TraceID,
	"SpanID":               ottlfuncs.SpanID,
	"IsMatch":              ottlfuncs.IsMatch,
	"Concat":               ottlfuncs.Concat,
	"Split":                ottlfuncs.Split,
	"keep_keys":            ottlfuncs.KeepKeys,
	"set":                  ottlfuncs.Set,
	"truncate_all":         ottlfuncs.TruncateAll,
	"limit":                ottlfuncs.Limit,
	"replace_match":        ottlfuncs.ReplaceMatch,
	"replace_all_matches":  ottlfuncs.ReplaceAllMatches,
	"replace_pattern":      ottlfuncs.ReplacePattern,
	"replace_all_patterns": ottlfuncs.ReplaceAllPatterns,
	"delete_key":           ottlfuncs.DeleteKey,
	"delete_matching_keys": ottlfuncs.DeleteMatchingKeys,
}

func Functions() map[string]interface{} {
	return registry
}
