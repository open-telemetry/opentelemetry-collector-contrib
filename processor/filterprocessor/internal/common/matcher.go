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
)

func CheckConditions[K any](ctx context.Context, tCtx K, statements []*ottl.Statement[K]) (bool, error) {
	for _, statement := range statements {
		_, metCondition, err := statement.Execute(ctx, tCtx)
		if err != nil {
			return false, err
		}
		if metCondition {
			return true, nil
		}
	}
	return false, nil
}
