{{/*
Default config for logging output
*/}}
{{- define "opentelemetry-collector.loggingOutputConfig" -}}
exporters:
  logging:
    logLevel: {{ .Values.output.logLevel | default "info" }}
service:
  pipelines:
    {{- if .Values.telemetry.metrics.enabled }}
    metrics:
      exporters: [logging]
    {{- end }}

    {{- if .Values.telemetry.traces.enabled }}
    traces:
      exporters: [logging]
    {{- end }}
{{- end }}
