# Redaction processor

Supported pipeline types: traces

This processor deletes span attributes that don't match a list of allowed span
attributes. It also masks span attribute values that match a blocked value
list. Span attributes that aren't on the allowed list are removed before any
value checks are done.

Typical use-cases:

* Prevent sensitive fields from accidentally leaking into traces
* Ensure compliance with legal, privacy, or security requirements

Please refer to [config.go](./config.go) for the config spec.

Examples:

```yaml
processors:
  redaction:
    # Flag to allow all span attribute keys. Setting this to true disables the
    # allowed_keys list. The list of blocked_values is applied regardless. If
    # you just want to block values, set this to true.
    allow_all_keys: false
    # Allowlist for span attribute keys. The list is designed to fail closed.
    # If allowed_keys is empty, no span attributes are allowed and all span
    # attributes are removed. To allow all keys, set allow_all_keys to true.
    # To allow the span attributes you know are good, add them to the list.
    allowed_keys:
      - description
      - group
      - id
      - name
    # BlockedValues is a list of regular expressions for blocking values of
    # allowed span attributes. Values that match are masked
    blocked_values:
      - "4[0-9]{12}(?:[0-9]{3})?" ## Visa credit card number
      - "(5[1-5][0-9]{14})"       ## MasterCard number
    # DryRun mode documents the changes the processor would make without
    # deleting your data. You can use it to confirm that the right span
    # attributes will be redacted
    dry_run: false
    # Summary controls the verbosity level of the diagnostic attributes that
    # the processor adds to the spans when it redacts or masks other
    # attributes. In some contexts a list of redacted attributes leaks
    # information, while it is valuable when integrating and testing a new
    # configuration. Possible values are `debug`, `info`, and `silent`.
    summary: debug
```

## Configuration

Refer to [config.yaml](./testdata/config.yaml) for detailed examples on using
the processor.

Only span attributes included on the list of allowed keys list are retained.
If `allowed_keys` is empty, then no span attributes are allowed. All span
attributes are removed in that case. To keep all span attributes, you should
explicitly set `allow_all_keys` to true.

`blocked_values` applies to the values of the allowed keys. If the value of an
allowed key matches the regular expression for a blocked value, the matching
part of the value is then masked with a fixed length of asterisks.

For example, if `notes` is on the list of allowed keys, then the `notes` span
attribute is retained. However, if there is a value such as a credit card
number in the `notes` field that matched a regular expression on the list of
blocked values, then that value is masked.
