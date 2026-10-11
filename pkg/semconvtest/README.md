# semconvtest

The package semconvtest validates Collector component telemetry against the
[OpenTelemetry semantic conventions](https://github.com/open-telemetry/semantic-conventions),
using [Weaver](https://github.com/open-telemetry/weaver) live-check. A test
hands its telemetry to this package; the package reports each semantic
convention violation as a test failure.

## How it works

Each `Test*` call performs the full cycle:

1. Start a Weaver `registry live-check` container (via testcontainers-go).
2. Send the provided telemetry to the container over OTLP gRPC.
3. Stop the listener; Weaver returns the live-check report in the response.
4. Fail the test with one error per violation-level finding.
5. Return the violation findings for further inspection.

## Requirements

- A running Docker daemon. Each `Test*` call starts one container.
- The `integration` build tag: the container machinery and the `Test*`
  functions compile only with `go test -tags integration`.
- Weaver v0.22.1 or newer (the first version with `--output=http`). The
  package rejects older semver versions with a clear error.

## Usage

Produce telemetry with your component, then hand it to the package together
with the `testing.TB`:

```go
//go:build integration

package myreceiver

import (
    "testing"

    "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"
)

func TestSemconvCompliance(t *testing.T) {
    metrics := produceMetricsWithMyComponent(t)
    semconvtest.TestMetrics(t, metrics)
}
```

The API offers one function per signal:

```go
func TestLogs(tb testing.TB, logs plog.Logs, opts ...WeaverOption) []PolicyFinding
func TestMetrics(tb testing.TB, metrics pmetric.Metrics, opts ...WeaverOption) []PolicyFinding
func TestTraces(tb testing.TB, traces ptrace.Traces, opts ...WeaverOption) []PolicyFinding
```

A full worked example lives in
[`internal/samplereceiver`](./internal/samplereceiver): a sample receiver that
emits a compliant `http.server.request.duration` metric, and a test that
validates it.

## Source of truth

By default the check runs against the newest published semantic-conventions
registry: Weaver downloads it when the container starts. As a consequence, a
test can start to fail without any code change, when the conventions evolve.
That is the compliance signal: the telemetry no longer matches the current
specification. For deterministic results, pin the registry with
`WithRegistry`.

The Weaver image itself does not float: the package pins a tested
`otel/weaver` version and updates it deliberately (Renovate opens a PR for
each new Weaver release). Use `WithVersion` to select a different version.

## Options

- `WithVersion(version string)`: select the `otel/weaver` image version.
  The default is the pinned, tested version; the minimum is v0.22.1. Tags
  that are not semver (for example, `latest`) pass through to Docker
  unchecked.
- `WithRegistry(registry string)`: set the semantic-conventions registry
  passed to Weaver's `--registry` flag. When unset, Weaver uses its default
  registry: the latest published semantic conventions.

## Report types

The report model (`LiveCheckReport`, `PolicyFinding`, and the
`ParseLiveCheckReport`, `HasViolations`, `GetViolations` helpers) compiles
without the `integration` tag, so report parsing is usable in any build.
