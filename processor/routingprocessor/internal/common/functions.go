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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/common"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/functions/tqlcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/functions/tqlotel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

var registry = map[string]interface{}{
	"IsMatch":              tqlcommon.IsMatch,
	"delete_key":           tqlotel.DeleteKey,
	"delete_matching_keys": tqlotel.DeleteMatchingKeys,
	// noop function, it is required since the parsing of conditions is not implemented yet,
	// see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/13545
	"route": func(_ []string) (tql.ExprFunc, error) {
		return func(ctx tql.TransformContext) interface{} {
			return true
		}, nil
	},
}

func Functions() map[string]interface{} {
	return registry
}
