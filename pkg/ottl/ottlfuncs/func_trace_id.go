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

type TraceIDArguments[K any] struct {
	Bytes []byte `ottlarg:"0"`
}

func NewTraceIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TraceID", &TraceIDArguments[K]{}, createTraceIDFunction[K])
}

func createTraceIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TraceIDArguments[K])

	if !ok {
		return nil, fmt.Errorf("TraceIDFactory args must be of type *TraceIDArguments[K]")
	}

	return traceID[K](args.Bytes)
}

func traceID[K any](bytes []byte) (ottl.ExprFunc[K], error) {
	if len(bytes) != 16 {
		return nil, errors.New("traces ids must be 16 bytes")
	}
	var idArr [16]byte
	copy(idArr[:16], bytes)
	id := pcommon.TraceID(idArr)
	return func(context.Context, K) (interface{}, error) {
		return id, nil
	}, nil
}
