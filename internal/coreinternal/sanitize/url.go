// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sanitize // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/sanitize"

import (
	"net/url"
	"regexp"
)

var (
	carriageReturnOrLineFeedRegexp = regexp.MustCompile(`\n|\r`)
)

// URL removes control characters from the URL parameter. This addresses CWE-117:
// https://cwe.mitre.org/data/definitions/117.html
func URL(unsanitized *url.URL) string {
	return carriageReturnOrLineFeedRegexp.ReplaceAllString(unsanitized.String(), "")
}
