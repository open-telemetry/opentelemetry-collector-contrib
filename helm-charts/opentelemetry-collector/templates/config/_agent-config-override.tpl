{{/*
Default config override for agent collector deamonset
*/}}
{{- define "opentelemetry-collector.agentConfigOverride" -}}
exporters:
  {{- if .Values.standaloneCollector.enabled }}
  otlp:
    endpoint: {{ include "opentelemetry-collector.fullname" . }}:55680
    insecure: true
  {{- end }}

service:
  pipelines:
    {{- if and .Values.telemetry.metrics.enabled .Values.standaloneCollector.enabled }}
    metrics:
      exporters: [otlp]
    {{- end }}

    {{- if and .Values.telemetry.traces.enabled .Values.standaloneCollector.enabled }}
    traces:
      exporters: [otlp]
    {{- end }}
{{- end }}
