{{/*
Default traces pipeline config
*/}}
{{- define "opentelemetry-collector.tracesPipelineConfig" -}}
receivers:
  jaeger:
    protocols:
      thrift_http:
        endpoint: 0.0.0.0:14268
      grpc:
        endpoint: 0.0.0.0:14250
  zipkin:
    endpoint: 0.0.0.0:9411
service:
  pipelines:
    traces:
      receivers: [otlp, jaeger, zipkin]
      processors: [memory_limiter, batch, queued_retry]
{{- end }}
