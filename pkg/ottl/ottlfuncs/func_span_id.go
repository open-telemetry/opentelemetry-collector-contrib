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

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type SpanIDArguments[K any] struct {
	Bytes []byte `ottlarg:"0"`
}

func NewSpanIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SpanID", &SpanIDArguments[K]{}, createSpanIDFunction[K])
}

func createSpanIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SpanIDArguments[K])

	if !ok {
		return nil, fmt.Errorf("SpanIDFactory args must be of type *SpanIDArguments[K]")
	}

	return spanID[K](args.Bytes)
}

func spanID[K any](bytes []byte) (ottl.ExprFunc[K], error) {
	if len(bytes) != 8 {
		return nil, errors.New("span ids must be 8 bytes")
	}
	var idArr [8]byte
	copy(idArr[:8], bytes)
	id := pcommon.SpanID(idArr)
	return func(context.Context, K) (interface{}, error) {
		return id, nil
	}, nil
}
