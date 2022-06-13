# Contributing

This guide is specific to the transform processor.  All guidelines in [Collector Contrib's CONTRIBUTING.MD](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md) must also be followed.

## New Functions

All new functions must be added via a new file.  Function files must start with `func_`.  Functions that are usable in multiple pipelines must be placed in `internal/common`.  Functions that are specific to a pipeline must be placed in `internal/<pipeline>`.

New functions must update the appropriate registry.  For common functions, update the registry in `internal/common/functions.go`.  For pipeline-specific functions, update the registry in `internal/<pipeline>/functions.go`

Unit tests must be added for all new functions.  Unit test files must start with `func_` and end in `_test`.  Unit tests must be placed in the same directory as the function.  Functions that are not specific to a pipeline should be tested independently of any specific pipeline. Functions that are specific to a pipeline should be tests against that pipeline.

All new functions should have integration tests added to any usable pipeline's `processing_test.go` tests.  The purpose of these tests is not to test the function's logic, but its ability to be used within a specific pipeline.  

## New Values

When adding new values to the grammar you must:

1. Update the `Value` struct with the new value.  This may also mean adding new token(s) to the lexer.
2. Update `NewFunctionCall` to be able to handle calling functions with this new value.
3. Update `NewGetter` to be able to handle the new value.
4. Add new unit tests.
