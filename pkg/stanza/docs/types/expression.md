# Expressions

Expressions give the config flexibility by allowing dynamic business logic rules to be included in static configs.
Most notably, expressions can be used to route log records and add new fields based on the contents of the log entry
being processed.

For more information, see the [expression language reference documentation](https://github.com/expr-lang/expr/blob/master/docs/language-definition.md).

Available to the expressions are a few special variables:
- `body` contains the entry's body
- `attributes` contains the entry's attributes
- `resource` contains the entry's resource
- `timestamp` contains the entry's timestamp
- `env()` is a function that allows you to read environment variables

## Examples

### Add a label from an environment variable

```yaml
- type: add
  field: attributes.stack
  value: 'EXPR(env("STACK"))'
```
