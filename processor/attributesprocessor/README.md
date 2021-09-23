# Attributes Processor

Supported pipeline types: traces, logs.

The attributes processor modifies attributes of a span. Please refer to
[config.go](./config.go) for the config spec.

It optionally supports the ability to [include/exclude spans](#includeexclude-spans).

It takes a list of actions which are performed in order specified in the config.
The supported actions are:
- `insert`: Inserts a new attribute in spans where the key does not already exist.
- `update`: Updates an attribute in spans where the key does exist.
- `upsert`: Performs insert or update. Inserts a new attribute in spans where the
  key does not already exist and updates an attribute in spans where the key
  does exist.
- `delete`: Deletes an attribute from a span.
- `hash`: Hashes (SHA1) an existing attribute value.
- `extract`: Extracts values using a regular expression rule from the input key
  to target keys specified in the rule. If a target key already exists, it will
  be overridden. Note: It behaves similar to the Span Processor `to_attributes`
  setting with the existing attribute as the source.

For the actions `insert`, `update` and `upsert`,
 - `key`  is required
 - one of `value` or `from_attribute` is required
 - `action` is required.
```yaml
  # Key specifies the attribute to act upon.
- key: <key>
  action: {insert, update, upsert}
  # Value specifies the value to populate for the key.
  # The type is inferred from the configuration.
  value: <value>

  # Key specifies the attribute to act upon.
- key: <key>
  action: {insert, update, upsert}
  # FromAttribute specifies the attribute from the span to use to populate
  # the value. If the attribute doesn't exist, no action is performed.
  from_attribute: <other key>
```

For the `delete` action,
 - `key` is required
 - `action: delete` is required.
```yaml
# Key specifies the attribute to act upon.
- key: <key>
  action: delete
```


For the `hash` action,
 - `key` is required
 - `action: hash` is required.
```yaml
# Key specifies the attribute to act upon.
- key: <key>
  action: hash
```


For the `extract` action,
 - `key` is required
 - `pattern` is required.
 ```yaml
 # Key specifies the attribute to extract values from.
 # The value of `key` is NOT altered.
- key: <key>
  # Rule specifies the regex pattern used to extract attributes from the value
  # of `key`.
  # The submatchers must be named.
  # If attributes already exist, they will be overwritten.
  pattern: <regular pattern with named matchers>
  action: extract

 ```

The list of actions can be composed to create rich scenarios, such as
back filling attribute, copying values to a new key, redacting sensitive information.
The following is a sample configuration.

```yaml
processors:
  attributes/example:
    actions:
      - key: db.table
        action: delete
      - key: redacted_span
        value: true
        action: upsert
      - key: copy_key
        from_attribute: key_original
        action: update
      - key: account_id
        value: 2245
      - key: account_password
        action: delete
      - key: account_email
        action: hash

```

Refer to [config.yaml](./testdata/config.yaml) for detailed
examples on using the processor.

## Include/Exclude Spans

The [attribute processor](README.md) and the [span processor](../spanprocessor/README.md) expose
the option to provide a set of properties of a span to match against to determine
if the span should be included or excluded from the processor. To configure
this option, under `include` and/or `exclude` at least `match_type` and one of
`services`, `span_names` or `attributes` is required.

Note: If both `include` and `exclude` are specified, the `include` properties
are checked before the `exclude` properties.

```yaml
{span, attributes}:
    # include and/or exclude can be specified. However, the include properties
    # are always checked before the exclude properties.
    {include, exclude}:
      # At least one of services, span_names or attributes must be specified.
      # It is supported to have more than one specified, but all of the specified
      # conditions must evaluate to true for a match to occur.

      # match_type controls how items in "services" and "span_names" arrays are
      # interpreted. Possible values are "regexp" or "strict".
      # This is a required field.
      match_type: {strict, regexp}

      # regexp is an optional configuration section for match_type regexp.
      regexp:
        # < see "Match Configuration" below >

      # services specify an array of items to match the service name against.
      # A match occurs if the span service name matches at least of the items.
      # This is an optional field.
      services: [<item1>, ..., <itemN>]

      # The span name must match at least one of the items.
      # This is an optional field.
      span_names: [<item1>, ..., <itemN>]

      # Attributes specifies the list of attributes to match against.
      # All of these attributes must match exactly for a match to occur.
      # This is an optional field.
      attributes:
          # Key specifies the attribute to match against.
        - key: <key>
          # Value specifies the exact value to match against.
          # If not specified, a match occurs if the key is present in the attributes.
          value: {value}
```

### Match Configuration

Some `match_type` values have additional configuration options that can be
specified. The `match_type` value is the name of the configuration section.
These sections are optional.

```yaml
# regexp is an optional configuration section for match_type regexp.
regexp:
  # cacheenabled determines whether match results are LRU cached to make subsequent matches faster.
  # Cache size is unlimited unless cachemaxnumentries is also specified.
  cacheenabled: <bool>
  # cachemaxnumentries is the max number of entries of the LRU cache; ignored if cacheenabled is false.
  cachemaxnumentries: <int>
```