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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/functions/tqlcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/functions/tqlotel"
)

var registry = map[string]interface{}{
	"TraceID":              tqlotel.TraceID,
	"SpanID":               tqlotel.SpanID,
	"IsMatch":              tqlcommon.IsMatch,
	"Concat":               tqlcommon.Concat,
	"keep_keys":            tqlotel.KeepKeys,
	"set":                  tqlcommon.Set,
	"truncate_all":         tqlotel.TruncateAll,
	"limit":                tqlotel.Limit,
	"replace_match":        tqlcommon.ReplaceMatch,
	"replace_all_matches":  tqlotel.ReplaceAllMatches,
	"replace_pattern":      tqlcommon.ReplacePattern,
	"replace_all_patterns": tqlotel.ReplaceAllPatterns,
	"delete_key":           tqlotel.DeleteKey,
	"delete_matching_keys": tqlotel.DeleteMatchingKeys,
}

func Functions() map[string]interface{} {
	return registry
}
