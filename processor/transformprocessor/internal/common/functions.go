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

var registry = map[string]interface{}{
	"TraceID":              traceID,
	"SpanID":               spanID,
	"IsMatch":              isMatch,
	"keep_keys":            keepKeys,
	"set":                  set,
	"truncate_all":         truncateAll,
	"limit":                limit,
	"replace_match":        replaceMatch,
	"replace_all_matches":  replaceAllMatches,
	"replace_pattern":      replacePattern,
	"replace_all_patterns": replaceAllPatterns,
	"delete_key":           deleteKey,
	"delete_matching_keys": deleteMatchingKeys,
}

func DefaultFunctions() map[string]interface{} {
	return registry
}
