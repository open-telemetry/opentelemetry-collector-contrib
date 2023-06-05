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

package redactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

type Config struct {

	// AllowAllKeys is a flag to allow all span attribute keys. Setting this
	// to true disables the AllowedKeys list. The list of BlockedValues is
	// applied regardless. If you just want to block values, set this to true.
	AllowAllKeys bool `mapstructure:"allow_all_keys"`

	// AllowedKeys is a list of allowed span attribute keys. Span attributes
	// not on the list are removed. The list fails closed if it's empty. To
	// allow all keys, you should explicitly set AllowAllKeys
	AllowedKeys []string `mapstructure:"allowed_keys"`

	// IgnoredKeys is a list of span attribute keys that are not redacted.
	// Span attributes in this list are allowed to pass through the filter
	// without being changed or removed.
	IgnoredKeys []string `mapstructure:"ignored_keys"`

	// BlockedValues is a list of regular expressions for blocking values of
	// allowed span attributes. Values that match are masked
	BlockedValues []string `mapstructure:"blocked_values"`

	// Summary controls the verbosity level of the diagnostic attributes that
	// the processor adds to the spans when it redacts or masks other
	// attributes. In some contexts a list of redacted attributes leaks
	// information, while it is valuable when integrating and testing a new
	// configuration. Possible values are `debug`, `info`, and `silent`.
	Summary string `mapstructure:"summary"`
}
