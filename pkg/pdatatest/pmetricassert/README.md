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

`WriteAssertionFile` expects semantically valid metrics. It normalizes valid
metrics into an assertion snapshot; it is not a validator for producer output.

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

### Attribute presence matcher

Attribute keys can use the `/exists: true` suffix when the attribute must be
present but its value is volatile:

```yaml
attributes:
  service.name: svc
  service.instance.id/exists: true
```

The attribute map remains exact: unexpected attributes still fail the
assertion, so omitting a key is the way to assert that it must not appear.
`/exists: true` is the only supported value; any other value is a schema
error.

### Attribute regex matcher

Attribute keys can use the `/regex` suffix when the attribute value is a
volatile string that must match a regular expression. The regular expression
must match the full attribute value:

```yaml
attributes:
  http.url/regex: 'http://127\.0\.0\.1:[0-9]+'
```

Regex matchers are supported for resource attributes and datapoint attributes.
The attribute map remains exact: unexpected attributes still fail the
assertion.

### Scope version matchers

The scope `version` field accepts the same `/exists` and `/regex` operators as
attributes, the assertion-file equivalent of `pmetrictest.IgnoreScopeVersion`.
The scope `name` is always matched exactly.

```yaml
scopes:
  - name: github.com/example/receiver
    version/exists: true               # present, any value
  - name: github.com/example/other
    version/regex: 'v[0-9]+\.[0-9]+\.[0-9]+'  # full-string match
```

Use at most one of `version:`, `version/exists:`, or `version/regex:` per
scope. `version/exists` accepts only `true`; any other value is a schema
error.

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
extensions beyond attribute `/exists`/`/regex` and scope `version`
`/exists`/`/regex` (`/include`, `/exclude`, `/all`, `/count`, `/approx`,
`/gt|gte|lt|lte`) and opt-in fields
(`IncludeValues()`, `IncludeTimestamps()`, `IncludeExemplars()`, type-specific
histogram fields) are tracked as follow-ups under that issue.
