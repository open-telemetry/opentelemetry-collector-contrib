// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"strconv"
	"strings"
)

type LuhnArguments[K any] struct {
	Target ottl.StringLikeGetter[K]
}

func NewLuhnFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Luhn", &LuhnArguments[K]{}, createLuhnFunction[K])
}

func createLuhnFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*LuhnArguments[K])

	if !ok {
		return nil, fmt.Errorf("LuhnFactory args must be of type *LuhnArguments[K]")
	}

	return luhnFunc(args.Target), nil
}

func luhnFunc[K any](target ottl.StringLikeGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		value, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if value == nil {
			return nil, fmt.Errorf("invalid input: %v", value)
		}

		// first trim all spaces
		trimmedNumber := strings.ReplaceAll(*value, " ", "")

		// return false if the value is an empty string
		if len(trimmedNumber) == 0 {
			return false, nil
		}

		// extract the check digit (the right most digit)
		checkDigit, err := strconv.Atoi(string(trimmedNumber[len(trimmedNumber)-1]))
		if err != nil {
			return nil, err
		}

		sum := 0
		alternate := true
		for i := len(trimmedNumber) - 2; i >= 0; i-- {
			n, err := strconv.Atoi(string(trimmedNumber[i]))
			if err != nil {
				return nil, err
			}

			if alternate {
				// double the digit
				n *= 2
				// subtract 9 if the number is greater than 9
				if n > 9 {
					n -= 9
				}
			}
			sum += n
			alternate = !alternate
		}
		// calculate the check sum
		actualChecksum := (10 - sum%10) % 10

		return actualChecksum == checkDigit, nil
	}
}
