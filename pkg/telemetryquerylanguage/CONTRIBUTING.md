# Contributing

This guide is specific to the Telemetry Query Language.  All guidelines in [Collector Contrib's CONTRIBUTING.MD](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md) must also be followed.

## New Values

When adding new values to the grammar you must:

1. Update the `Value` struct with the new value.  This may also mean adding new token(s) to the lexer.
2. Update `NewFunctionCall` to be able to handle calling functions with this new value.
3. Update `NewGetter` to be able to handle the new value.
4. Add new unit tests.

## New Functions

All new functions must be added via a new file.  Function files must start with `func_`.  Functions that are usable for any underlying telemetry must be placed in `functions/tqlcommon`.  Functions that are specific to otel must be placed in `functions/tqlotel`.

Unit tests must be added for all new functions.  Unit test files must start with `func_` and end in `_test`.  Unit tests must be placed in the same directory as the function.  Functions that are not specific to a pipeline should be tested independently of any specific pipeline. Functions that are specific to a pipeline should be tests against that pipeline.