# Contributing

This guide is specific to the Telemetry Query Language.  All guidelines in [Collector Contrib's CONTRIBUTING.MD](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md) must also be followed.

## New Values

When adding new values to the grammar you must:

1. Update the `Value` struct with the new value.  This may also mean adding new token(s) to the lexer.
2. Update `NewFunctionCall` to be able to handle calling functions with this new value.
3. Update `NewGetter` to be able to handle the new value.
4. Add new unit tests.