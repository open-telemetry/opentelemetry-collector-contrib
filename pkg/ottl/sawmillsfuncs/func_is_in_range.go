package sawmillsfuncs

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// IsInRangeArguments holds the arguments for the IsInRange function.
// K represents the type of the context that will be available at runtime.
type IsInRangeArguments[K any] struct {
	// Target is a getter that returns a numeric value to check.
	// The value can be a float64, int64, or a string representation of a number.
	Target ottl.FloatLikeGetter[K]
	// Min is a getter that returns the minimum value of the range (inclusive).
	Min ottl.FloatLikeGetter[K]
	// Max is a getter that returns the maximum value of the range (inclusive).
	Max ottl.FloatLikeGetter[K]
}

// NewIsInRangeFactory creates a new factory for the IsInRange function.
func NewIsInRangeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsInRange", &IsInRangeArguments[K]{}, createIsInRangeFunction[K])
}

// createIsInRangeFunction creates a function that checks if a value is within a specified range.
// This function performs two phases of validation:
//
// 1. Validation Phase (during function creation):
//   - Validates that all getters (Target, Min, Max) are properly configured to handle numeric values
//   - Uses a zero value of type K to perform a "dry run" of the getters
//   - Catches configuration errors early, before any runtime execution
//   - Ensures Min is less than or equal to Max (using validation values)
//
// 2. Runtime Phase (during function execution):
//   - Uses the actual context to get real-time values for Target, Min, and Max
//   - All three values are re-evaluated on each function call based on the runtime context
//   - Checks if the target value falls within the range [min, max]
//   - Values can change between function calls based on the runtime context
func createIsInRangeFunction[K any](
	_ ottl.FunctionContext,
	oArgs ottl.Arguments,
) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsInRangeArguments[K])
	if !ok {
		return nil, fmt.Errorf("IsInRangeFactory args must be of type *IsInRangeArguments[K]")
	}

	// Validate that all required arguments are provided
	if args.Target == nil {
		return nil, fmt.Errorf("target is required")
	}
	if args.Min == nil {
		return nil, fmt.Errorf("min is required")
	}
	if args.Max == nil {
		return nil, fmt.Errorf("max is required")
	}

	// Validation Phase:
	// Here we validate the configuration of the getters using a zero value of K.
	// This ensures the getters are properly set up to handle numeric values,
	// even though the actual values will be determined at runtime.
	ctx := context.Background()

	// Validate target type
	target, err := args.Target.Get(ctx, *new(K))
	if err != nil {
		return nil, fmt.Errorf("target must be a number")
	}
	if target == nil {
		return nil, fmt.Errorf("target value is nil")
	}

	// Validate min type
	min, err := args.Min.Get(ctx, *new(K))
	if err != nil {
		return nil, fmt.Errorf("min must be a number")
	}
	if min == nil {
		return nil, fmt.Errorf("min value is nil")
	}

	// Validate max type
	max, err := args.Max.Get(ctx, *new(K))
	if err != nil {
		return nil, fmt.Errorf("max must be a number")
	}
	if max == nil {
		return nil, fmt.Errorf("max value is nil")
	}

	// Validate that min <= max using the validation values
	if *min > *max {
		return nil, fmt.Errorf("min must be less than or equal to max")
	}

	// Runtime Phase:
	// Return a function that will be executed at runtime with actual context values.
	// The getters may return different values on each call based on the runtime context.
	return func(ctx context.Context, tCtx K) (any, error) {
		// Get the current target value from the runtime context
		target, err := args.Target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if target == nil {
			return nil, fmt.Errorf("target value is nil")
		}

		// Get the current min value from the runtime context
		min, err := args.Min.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if min == nil {
			return nil, fmt.Errorf("min value is nil")
		}

		// Get the current max value from the runtime context
		max, err := args.Max.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if max == nil {
			return nil, fmt.Errorf("max value is nil")
		}

		// Check if target is in range [min, max]
		return *target >= *min && *target <= *max, nil
	}, nil
}
