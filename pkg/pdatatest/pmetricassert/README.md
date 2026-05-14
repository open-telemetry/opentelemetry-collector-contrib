# pmetricassert

`pmetricassert` provides an MTS-focused assertion framework for `pmetric.Metrics`,
based on an editable YAML snapshot describing what the test cares about: metric
identity and the set of datapoint attribute permutations.

It is an alternative to `pmetrictest.CompareMetrics`. Use whichever fits the
test:

| Concern                                       | Use                            |
|-----------------------------------------------|--------------------------------|
| "These metrics, with these attribute permutations, are produced." | `pmetricassert.AssertMetrics` |
| "The full pdata tree (including values, timestamps, descriptions, exemplars) matches byte-for-byte." | `pmetrictest.CompareMetrics`  |

The trade-off mirrors the proposal in
[#48079](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/48079):
`CompareMetrics` starts from a full pdata tree and lets the test opt out of
volatile fields; `pmetricassert` starts from identity and lets the test opt in
to additional fields.

## What is asserted

By default the YAML snapshot pins:

- resource attributes,
- scope name and version,
- metric name, type, unit, temporality, monotonicity,
- the set of datapoint attribute permutations (the MTS identity).

The following are ignored:

- datapoint values, timestamps, start timestamps,
- exemplars, metric descriptions,
- the order of resources, scopes, metrics, and datapoints,
- batch boundaries — multiple `ResourceMetrics` / `ScopeMetrics` / `Metric`
  entries with the same identity are normalized before comparison.

## Typical usage

```go
func TestScraper(t *testing.T) {
    actualMetrics, err := scraper.scrape(t.Context())
    require.NoError(t, err)

    expectedFile := filepath.Join("testdata", "scraper", "metrics.assert.yaml")
    // To regenerate: uncomment, run the test once, re-comment.
    // require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedFile, actualMetrics))

    require.NoError(t, pmetricassert.AssertMetrics(expectedFile, actualMetrics))
}
```

## YAML schema

```yaml
version: 1
signal: metrics
resources:
  - attributes:
      service.name: svc
    scopes:
      - name: github.com/example/receiver
        version: v0.0.1
        metrics:
          - name: svc.active
            type: gauge
            unit: "1"
          - name: svc.requests
            type: sum
            unit: "{requests}"
            temporality: cumulative
            monotonic: true
            datapoints:
              - attributes:
                  method: GET
              - attributes:
                  method: POST
```

### Shorthand: single empty-attribute datapoint

A metric with exactly one datapoint that has no attributes can omit
`datapoints:` entirely. The two forms are equivalent:

```yaml
- name: svc.active
  type: gauge
  unit: "1"
```

```yaml
- name: svc.active
  type: gauge
  unit: "1"
  datapoints:
    - {}
```

This matters because most single-series metrics (counters, current-value
gauges) fall into this shape, and dropping the `datapoints:` key keeps the
common case readable. The shorthand relies on the invariant that a `Metric`
must contain at least one datapoint; see
[#48106](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/48106).

## Roadmap

This is the identity-only subset of the grammar in #48079. Operator-suffix
extensions (`/include`, `/exclude`, `/all`, `/count`, `/regex`, `/exists`,
`/approx`, `/gt|gte|lt|lte`) and opt-in fields (`IncludeValues()`,
`IncludeTimestamps()`, `IncludeExemplars()`, type-specific histogram fields)
are tracked as follow-ups under that issue.
