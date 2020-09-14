{{/*
Default output for agent collector pointed to standalone collector
*/}}
{{- define "opentelemetry-collector.standaloneCollectorOutputConfig" -}}
exporters:
  otlp:
    endpoint: {{ include "opentelemetry-collector.fullname" . }}:55680
    insecure: true
service:
  pipelines:
    {{- if .Values.telemetry.metrics.enabled }}
    metrics:
      exporters: [otlp]
    {{- end }}

    {{- if .Values.telemetry.traces.enabled }}
    traces:
      exporters: [otlp]
    {{- end }}
{{- end }}
