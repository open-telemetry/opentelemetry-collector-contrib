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

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

func WithStandardFunctions[K any]() ottl.Option {
	return func(fm ottl.FunctionMap) {
		fm["keep_keys"] = KeepKeys[K]
		fm["set"] = Set[K]
		fm["truncate_all"] = TruncateAll[K]
		fm["limit"] = Limit[K]
		fm["replace_match"] = ReplaceMatch[K]
		fm["replace_all_matches"] = ReplaceAllMatches[K]
		fm["replace_pattern"] = ReplacePattern[K]
		fm["replace_all_patterns"] = ReplaceAllPatterns[K]
		fm["delete_key"] = deleteKey[K]
		fm["delete_matching_keys"] = deleteMatchingKeys[K]
		fm["merge_maps"] = MergeMaps[K]
	}
}

func WithFactoryFunctions[K any]() ottl.Option {
	return func(fm ottl.FunctionMap) {
		fm["TraceID"] = TraceID[K]
		fm["SpanID"] = SpanID[K]
		fm["IsMatch"] = isMatch[K]
		fm["Concat"] = Concat[K]
		fm["Split"] = Split[K]
		fm["Int"] = Int[K]
		fm["ConvertCase"] = ConvertCase[K]
		fm["ParseJSON"] = ParseJSON[K]
	}
}
