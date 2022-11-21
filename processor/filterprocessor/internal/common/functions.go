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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func Functions[K any]() map[string]interface{} {
	return map[string]interface{}{
		"TraceID":     ottlfuncs.TraceID[K],
		"SpanID":      ottlfuncs.SpanID[K],
		"IsMatch":     ottlfuncs.IsMatch[K],
		"Concat":      ottlfuncs.Concat[K],
		"Split":       ottlfuncs.Split[K],
		"Int":         ottlfuncs.Int[K],
		"ConvertCase": ottlfuncs.ConvertCase[K],
		"drop": func() (ottl.ExprFunc[K], error) {
			return func(context.Context, K) (interface{}, error) {
				return true, nil
			}, nil
		},
	}
}
