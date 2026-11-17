# Configuration compatibility and migration

The File Log Receiver follows the Collector's [configuration stability requirements](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#beta).
The receiver is currently beta. Backward incompatible configuration changes should be rare
and must provide migration instructions. New required options must have sensible defaults.
After graduation to stable, configuration must remain compatible between minor versions,
except for critical security issues with a documented migration path and upgrade period.

Renamed or removed options are deprecated with a WARN message linking here. If the warning
first ships in release N, the option remains available in N+1 and may be feature gated in
N+2. Neither the option nor its warning may be removed before both N+2 and six months after
deprecation. An option being removed may become non-operational when deprecated.

## Explicit ordering limit

Warning introduced: next release containing this change.

When `ordering_criteria.sort_by` is configured without `ordering_criteria.top_n`, the
receiver currently uses a limit of one file and logs a migration warning. This remains
the default behavior while the alpha `filelog.requireExplicitTopN` feature gate is disabled.
Enabling that gate rejects configurations that omit `top_n` when sorting is configured.
The gate was introduced in v0.159.0; its introduction is not the start of the warning's
deprecation period. No removal release is scheduled here.

To preserve the existing file selection limit, add `top_n: 1`:

```yaml
receivers:
  file_log:
    include: [/var/log/*.log]
    ordering_criteria:
      top_n: 1
      regex: '(?P<name>.*)'
      sort_by:
        - regex_key: name
          sort_type: alphabetical
          ascending: true
```

Use a positive integer to select another limit, or `top_n: 0` to match all files.
Explicit zero means all files with either state of the feature gate. Negative values are
invalid. No `top_n` is needed when `sort_by` is absent. Setting an explicit limit removes
the warning and works with either state of `filelog.requireExplicitTopN`.

When upgrading from before v0.159.0, an explicit `top_n: 0` used to select one file.
Set `top_n: 1` before upgrading if that is the behavior you need to preserve. This
change already shipped in v0.159.0 and is independent of the feature gate.

## Receiver type name

Alias deprecated: v0.149.0. Receiver-specific warning with this migration link:
next release containing this change. No alias removal release is scheduled.

Use `file_log` for new configurations. The previous `filelog` type remains a deprecated
alias. When migrating, change both the receiver key (for example, `filelog/app` to
`file_log/app`) and its references in `service.pipelines.logs.receivers`. Keep the instance
name after `/` unchanged. Feature gate identifiers such as `filelog.requireExplicitTopN`
retain their existing names.

The receiver normalizes underscores out of the component type when requesting persistent
storage, so `filelog/app` and `file_log/app` use the same storage namespace. Preserve the
instance name and storage configuration to retain offsets during this rename.

## Timestamp ordering configuration

Use `ordering_criteria.sort_by[].layout` for a timestamp's strptime layout, such as
`'%Y-%m-%d'`. Earlier versions of the README incorrectly called this key `format`;
the implementation uses `layout`. This is a documentation correction, not a renamed
configuration field. Similarly, select modification-time ordering with
`sort_by: [{sort_type: mtime}]`, not `ordering_criteria.mode`.

## Older configurations

These changes predate the receiver's beta configuration contract. They remain relevant
when migrating examples from older releases:

* Since v0.60.0, durations require explicit units. For example, replace `poll_interval: 1`
  with `poll_interval: 1s`.
* Since v0.64.0, the `converter` block is no longer configurable. Remove it. Conversion
  to OpenTelemetry log records is automatic; use `operators` to parse or transform entries
  and `retry_on_failure` for downstream retry behavior.

See the [changelog](../../CHANGELOG.md) for release-specific behavior changes and the
[operator documentation](../../pkg/stanza/docs/operators/README.md) for options belonging
to individual parsing and transformation operators.
