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

The check runs against a pinned pair, and both parts update deliberately:

- The registry: the package pins a released semantic-conventions version
  and passes it to Weaver. Renovate opens a PR for each new release; new
  conventions arrive in that PR, and a test that fails there shows
  telemetry that no longer matches them.
- The tool: the package pins a tested `otel/weaver` image version, also
  updated through Renovate PRs.

A pinned pair is compatible by construction: a published registry release
was built with an already-released Weaver. An unpinned registry can require
a newer Weaver than the pinned one at any time, which fails every test at
container start. Use the options below to select other versions.

## Options

- `WithVersion(version string)`: select the `otel/weaver` image version.
  The default is the pinned, tested version; the minimum is v0.22.1. Tags
  that are not semver (for example, `latest`) pass through to Docker
  unchecked.
- `WithRegistry(registry string)`: set the semantic-conventions registry
  passed to Weaver's `--registry` flag. The default is the pinned release.
  An empty string restores Weaver's own default: the unreleased registry on
  semantic-conventions main.

## Report types

The report model (`LiveCheckReport`, `PolicyFinding`, and the
`ParseLiveCheckReport`, `HasViolations`, `GetViolations` helpers) compiles
without the `integration` tag, so report parsing is usable in any build.
