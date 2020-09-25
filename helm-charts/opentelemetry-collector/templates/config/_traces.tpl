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
exporters:
  {{ .Values.telemetry.traces.exporter.type }}:
    {{- .Values.telemetry.traces.exporter.config | toYaml | nindent 4 }}
service:
  pipelines:
    traces:
      receivers: [otlp, jaeger, zipkin]
      processors: [memory_limiter, batch, queued_retry]
      exporters: [{{ .Values.telemetry.traces.exporter.type }}]
{{- end }}
