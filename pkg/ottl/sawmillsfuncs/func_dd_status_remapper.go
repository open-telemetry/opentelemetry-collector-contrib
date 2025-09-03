package sawmillsfuncs

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DdStatusRemapperArguments[K any] struct {
	Target ottl.Getter[K]
}

func NewDdStatusRemapperFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory(
		"DdStatusRemapper",
		&DdStatusRemapperArguments[K]{},
		createDdStatusRemapperFunction[K],
	)
}

func createDdStatusRemapperFunction[K any](
	_ ottl.FunctionContext,
	oArgs ottl.Arguments,
) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DdStatusRemapperArguments[K])

	if !ok {
		return nil, fmt.Errorf(
			"DdStatusRemapperFactory args must be of type *DdStatusRemapperArguments[K]",
		)
	}

	return statusRemapper(args.Target)
}

func statusRemapper[K any](target ottl.Getter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		var stringValue string

		switch v := val.(type) {
		case []byte:
			stringValue = string(v)
		case *string:
			stringValue = *v
		case string:
			stringValue = v
		case pcommon.ByteSlice:
			stringValue = string(v.AsRaw())
		case *pcommon.ByteSlice:
			stringValue = string(v.AsRaw())
		case pcommon.Value:
			stringValue = v.AsString()
		case *pcommon.Value:
			stringValue = v.AsString()
		default:
			return nil, nil
		}

		if stringValue == "" {
			return "info", nil
		}

		if levelInt, err := strconv.Atoi(stringValue); err == nil {
			switch levelInt {
			case 0:
				return "emerg", nil
			case 1:
				return "alert", nil
			case 2:
				return "critical", nil
			case 3:

				return "error", nil
			case 4:
				return "warning", nil
			case 5:
				return "notice", nil
			case 6:
				return "info", nil
			case 7:
				return "debug", nil
			default:
				return "info", nil
			}
		}

		levelLower := strings.ToLower(stringValue)

		switch {
		case strings.HasPrefix(levelLower, "emerg") || strings.HasPrefix(levelLower, "f"):
			return "emerg", nil
		case strings.HasPrefix(levelLower, "a"):
			return "alert", nil
		case strings.HasPrefix(levelLower, "c"):
			return "critical", nil
		case strings.HasPrefix(levelLower, "e") && !strings.HasPrefix(levelLower, "emerg"):
			return "error", nil
		case strings.HasPrefix(levelLower, "w"):
			return "warning", nil
		case strings.HasPrefix(levelLower, "n"):
			return "notice", nil
		case strings.HasPrefix(levelLower, "i"):
			return "info", nil
		case strings.HasPrefix(levelLower, "d") || strings.HasPrefix(levelLower, "trace") || strings.HasPrefix(levelLower, "verbose"):
			return "debug", nil
		case strings.HasPrefix(levelLower, "o") || strings.HasPrefix(levelLower, "s") || levelLower == "ok" || levelLower == "success":
			return "ok", nil
		default:
			return "info", nil
		}
	}, nil
}
