# Expr language

Expressions give the config flexibility by allowing dynamic business logic rules to be included in static configs.
Most notably, expressions can be used to filter logs and metrics based on content and properties of processed records.

For reference documentation of the expression language,
see [here](https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md).

## Metrics

For metrics the following variables as available:

| Name       | [Type][type] | Description         |
|------------|--------------|---------------------|
| MetricName | string       | Name of the Metric. |

In addition the following functions are available:

| Name     | Signature               | Description                                                  |
|----------|-------------------------|--------------------------------------------------------------|
| HasLabel | func(key string) bool   | Returns true if the Metric has given label, otherwise false. |
| Label    | func(key string) string | Returns value of given label name if exists, otherwise `""`  |

## Logs

For logs, the following variables are available:

| Name           | [Type][type] | Description                        |
|----------------|--------------|------------------------------------|
| Body           | string       | Stringified Body of the LogRecord. |
| Name           | string       | Name of the LogRecord.             |
| SeverityNumber | number       | SeverityNumber of the LogRecord.   |
| SeverityText   | string       | SeverityText of the LogRecord.     |

[type]: https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md#supported-literals
