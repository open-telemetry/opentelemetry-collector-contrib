{{/*
Default config override for agent collector deamonset
*/}}
{{- define "opentelemetry-collector.agentConfigOverride" -}}
receivers:
  {{- if and .Values.telemetry.metrics.enabled .Values.telemetry.metrics.hostMetricsEnabled }}
  hostmetrics:
    scrapers:
      cpu:
      disk:
      filesystem:
      memory:
      network:
      load:
      processes:
      swap:
      process:
  {{- end }}

exporters:
  {{- if .Values.standaloneCollector.enabled }}
  otlp:
    endpoint: {{ include "opentelemetry-collector.fullname" . }}:55680
    insecure: true
  {{- end }}

service:
  pipelines:
    {{- if .Values.telemetry.metrics.enabled }}
    metrics:
      receivers: 
        - prometheus
        {{- if and .Values.telemetry.metrics.hostMetricsEnabled }}
        - hostmetrics
        {{- end }}

      {{- if .Values.standaloneCollector.enabled }}
      exporters: [otlp]
      {{- end }}
    {{- end }}

    {{- if and .Values.telemetry.traces.enabled .Values.standaloneCollector.enabled }}
    traces:
      exporters: [otlp]
    {{- end }}
{{- end }}
