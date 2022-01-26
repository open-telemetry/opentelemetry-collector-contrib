# Regex Processor

Supported pipeline types: logs

The regex processor modifies the body of log records according to regular expressions exposed in sed format.

The processor accepts multiple regular expressions. They are executed in order against the body of the log record.

Example configuration:
```yaml
receivers:
  otlp:
    protocols:
      grpc:

exporters:
  nop

processors:
  regex:
    logs:
      patterns:
        - s/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[m|K]//g 
        - s/42/1337/g
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [regex]
      exporters: [nop]
```

This processor will perform the operations in order for all logs.

1) Remove all bash color codes such as `\e[31mRed Text\e[0m` is transformed to `Red Text`
2) Replace all occurrences of `42` with `1337`