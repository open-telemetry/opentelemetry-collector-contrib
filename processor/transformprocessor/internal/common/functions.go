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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/functions/ottlcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/functions/ottlotel"
)

var registry = map[string]interface{}{
	"TraceID":              ottlotel.TraceID,
	"SpanID":               ottlotel.SpanID,
	"IsMatch":              ottlcommon.IsMatch,
	"Concat":               ottlcommon.Concat,
	"Split":                ottlotel.Split,
	"keep_keys":            ottlotel.KeepKeys,
	"set":                  ottlcommon.Set,
	"truncate_all":         ottlotel.TruncateAll,
	"limit":                ottlotel.Limit,
	"replace_match":        ottlcommon.ReplaceMatch,
	"replace_all_matches":  ottlotel.ReplaceAllMatches,
	"replace_pattern":      ottlcommon.ReplacePattern,
	"replace_all_patterns": ottlotel.ReplaceAllPatterns,
	"delete_key":           ottlotel.DeleteKey,
	"delete_matching_keys": ottlotel.DeleteMatchingKeys,
}

func Functions() map[string]interface{} {
	return registry
}
