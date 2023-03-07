// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lumigoauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/lumigoauthextension"

import (
	"fmt"
	"regexp"
)

func ValidateLumigoToken(token string) error {
	if matched, err := regexp.MatchString(`t_[[:xdigit:]]{21}`, token); err != nil || !matched {
		return fmt.Errorf(
			"the Lumigo token does not match the expected structure of Lumigo tokens: " +
				"it should be `t_` followed by of 21 alphanumeric characters; see https://docs.lumigo.io/docs/lumigo-tokens " +
				"for instructions on how to retrieve your Lumigo token")
	}

	return nil
}
