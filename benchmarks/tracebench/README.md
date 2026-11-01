# Configurable trace-pipeline benchmark

This benchmark decodes synthetic OTLP requests, runs real Collector processors,
computes Datadog APM statistics before sampling, applies the real probabilistic
sampler, runs sampled-stage processors, and encodes the surviving traces.

The default workload is an arbitrary public example, **not production data or a
production calibration**. Organization-specific workload specifications,
processor configurations, telemetry, profiles and comparison reports belong
outside this repository. Do not commit them here.

## Run

Requires Linux and Go 1.26. From this directory:

```sh
GOWORK=off go test ./...
GOWORK=off go test -race ./...
GOWORK=off go test -run '^$' -bench '^BenchmarkTracePipeline$' -benchmem -benchtime=256x
```

Dependencies are pinned to Collector v0.164.1 components and official pdata,
with the local OTTL implementation.
This avoids confounding an implementation comparison with a distribution upgrade.

## Private configuration and validation

Set `TRACEBENCH_SPEC` to an external JSON file. Its schema follows `Spec`, `Class`,
`Batch` and `Stage` in [workload.go](workload.go):

- `classes` specify weights, span kinds, instrumentation scopes, attribute values,
  filler-attribute counts/lengths and spans per resource.
- `batches` specify **request weights**, request sizes, a fixed class index or
  `-1` for the class mixture, protobuf/JSON encoding and gzip.
- `bank` requests are generated with a fixed seed. Batch selection is stratified
  and shuffled to reduce mixture error. Use an iteration count divisible by
  `bank` so both variants consume the same workload rather than different prefixes.
- `trace_spans` groups consecutive input spans into a root and direct children.
  It defaults to one: every span is an independent root. Test other values when
  trace topology is unknown; topology affects APM statistics and allocations.
- `common` and `sampled` stages accept inline component configuration or an
  external YAML `file`, resolved by the Collector's file/environment providers.
- `sample_percent` controls the actual hash sampler. `stats` enables real APM
  statistics; `stats_config` accepts connector configuration.

```sh
TRACEBENCH_SPEC=/private/scenario.json GOWORK=off \
  go test -run '^TestConfiguredCensus$' -v
TRACEBENCH_SPEC=/private/scenario.json GOWORK=off \
  go test -run '^$' -bench '^BenchmarkTracePipeline$' -benchmem -benchtime=512x
```

The census checks the actual generated mix, batch sizes, wire bytes, surviving
spans and per-stage counters. Benchmark metrics include process CPU and allocated
bytes per incoming span, input wire bytes/span, sampling survival and exclusive
stage elapsed time/span, including a separate decode-time control. Setup and load
generation are excluded. Process CPU
includes background GC and APM work; do **not** add a separate GC-saving estimate.

Set `TRACEBENCH_CPU_PROFILE=/private/run.pprof` to profile only the measured
pipeline, excluding fixture generation and component construction. Use separate
unprofiled repetitions for timing comparisons.

For A/B tests, use a Go source overlay or a separate binary to change only the
implementation under test, keeping inputs and dependency versions identical.
Interleave repetitions, retain raw observations, and report regression cases.

## Reproduce an individual source improvement

[compare.py](compare.py) uses Python 3's standard library, Git and Go. Run it
from this directory, with `--output` outside the source tree. It copies the
baseline dependencies before applying only the named production-file diff,
without editing the shared Go module cache or upgrading dependencies.

It builds separate binaries, checks both workload/stage/survival censuses,
warms up both variants and runs shuffled A/B and A/A phases with alternating
ABBA/BAAB blocks. The optional `--pgo` flag first trains on the unchanged
baseline and then uses **that same public profile for both binaries**. No CPU
profiling occurs during the measured comparison.

Fetch the relevant fork's source branch into a local checkout first:

```sh
git -C /path/to/opentelemetry-collector fetch \
  https://github.com/githubnext/opentelemetry-collector.git \
  oss/pdata-map-get:refs/remotes/githubnext/oss/pdata-map-get
git -C /path/to/opentelemetry-collector-contrib fetch \
  https://github.com/githubnext/opentelemetry-collector-contrib.git \
  oss/ottl-literal-regex:refs/remotes/githubnext/oss/ottl-literal-regex
git -C /path/to/datadog-agent fetch \
  https://github.com/githubnext/datadog-agent.git \
  oss/kind-aware-operation-name:refs/remotes/githubnext/oss/kind-aware-operation-name \
  oss/batched-span-attribute-tags:refs/remotes/githubnext/oss/batched-span-attribute-tags
```

Each comparison is independent; do not stack patches or add their percentages:

```sh
python3 compare.py --candidate map-get \
  --repository /path/to/opentelemetry-collector \
  --base 302c933caa8c66f7ebf35ea42238b261bc3e5fc6 \
  --head refs/remotes/githubnext/oss/pdata-map-get \
  --pgo --blocks 8 --iterations 256 --output /tmp/tracebench-map-get

python3 compare.py --candidate literal-regex \
  --repository /path/to/opentelemetry-collector-contrib \
  --base a1cd60723506d875a7a4a94b525b9615c11f08fb \
  --head refs/remotes/githubnext/oss/ottl-literal-regex \
  --pgo --blocks 8 --iterations 256 --output /tmp/tracebench-literal-regex

python3 compare.py --candidate kind-aware \
  --repository /path/to/datadog-agent \
  --base 83b6d6e01408fb131211a8f10678f83ef7d64dc9 \
  --head refs/remotes/githubnext/oss/kind-aware-operation-name \
  --pgo --blocks 8 --iterations 256 --output /tmp/tracebench-kind-aware

python3 compare.py --candidate batched-tags \
  --repository /path/to/datadog-agent \
  --base 83b6d6e01408fb131211a8f10678f83ef7d64dc9 \
  --head refs/remotes/githubnext/oss/batched-span-attribute-tags \
  --pgo --blocks 8 --iterations 256 --output /tmp/tracebench-batched-tags
```

Each new output directory retains source patches/hashes, build logs, module
graphs, public training profile, census, raw observations and `result.json`.
Positive `less_cpu_percent` means less process CPU. Positive
`allocated_bytes_change_percent` means more allocated bytes. Intervals use
paired geometric block ratios and a circular block bootstrap of length four
with 5,000 draws. A/A intervals diagnose same-binary host variability; the
script does not subtract that variability or certify acceptance.

The public example exercises literal-key rewriting, Map.Get, operation naming
and APM statistics. Its client-only composition may give little benefit for
kind-aware naming, which skips irrelevant families for other kinds. Extend the
workload using the documented schema and publish the exact non-sensitive
scenario if claiming an effect. Add `--spec /path/to/scenario.json` to run an
external calibration; use an iteration count divisible by its `bank`.

**No fixed percentage or typical-workflow benefit is promised.** Results depend
on span kinds, attribute maps, stage configuration, compiler/PGO and hardware.
The arbitrary public example is not an internal production calibration. Census
equivalence covers input composition, counters and survivors, not every output
attribute or derived APM metric. Run the source proposal's correctness tests
as well. Repeat on a quiet host, report the workload and controls, and compare
PGO-on before evaluating an additional source improvement.

## Limits

This is a receiver-codec/pipeline benchmark, not a complete Collector deployment.
It does not include HTTP/TLS/auth, Kubernetes API lookup/watch, routing/fanout,
network/exporter queues, Datadog trace-payload translation or internal scraping.
The output encoder is OTLP protobuf, not the Datadog exporter. APM ingestion is
measured, but background metric flush cost and cardinality need separate checks.
Requests are processed serially. The byte bank remains in the Go heap and its
trace IDs are reused between iterations; neither models a live receiver's heap
or trace cardinality. A run must finish before the generated span timestamps age
out of the APM statistics window.
Do not silently pad CPU or allocations to force a match.

Aggregate counters do not identify original attribute maps, collisions, compression,
resource grouping or unsampled span-kind composition. State inferred parameters,
test plausible alternatives and validate held-out counters. Matching an input
rate or a single mean does not establish full-fleet representativeness.
