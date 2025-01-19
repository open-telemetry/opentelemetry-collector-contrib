# pkg/sampling

## Overview

This package contains utilities for parsing and interpreting the W3C
[TraceState](https://www.w3.org/TR/trace-context/#tracestate-header)
and all sampling-relevant fields specified by OpenTelemetry that may
be found in the OpenTelemetry section of the W3C TraceState.

This package implements the draft specification in [OTEP
235](https://github.com/open-telemetry/oteps/pull/235), which
specifies two fields used by the OpenTelemetry consistent probability
sampling scheme.

These are:

- `th`: the Threshold used to determine whether a TraceID is sampled
- `rv`: an explicit randomness value, which overrides randomness in the TraceID

[OTEP 235](https://github.com/open-telemetry/oteps/pull/235) contains
details on how to interpret these fields.  The are not meant to be
human readable, with a few exceptions.  The tracestate entry `ot=th:0`
indicates 100% sampling.
