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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/multierr"
)

func MaskResourceAttributeValue(res pcommon.Resource, attr string) {
	if _, ok := res.Attributes().Get(attr); ok {
		res.Attributes().Remove(attr)
	}
}

// AddErrPrefix adds a prefix to every multierr error.
func AddErrPrefix(prefix string, in error) error {
	var out error
	for _, err := range multierr.Errors(in) {
		out = multierr.Append(out, fmt.Errorf("%s: %w", prefix, err))
	}
	return out
}
