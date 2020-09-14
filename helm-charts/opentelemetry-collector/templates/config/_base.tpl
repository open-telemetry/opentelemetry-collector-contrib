{{/*
Base configuration for OpenTelemetry Collector
*/}}
{{- define "opentelemetry-collector.baseConfig" -}}
extensions:
  health_check: {}

receivers:
  otlp:
    protocols:
      grpc:
      http:

processors:
  queued_retry: {}
  batch: {}

  memory_limiter:
    # check_interval is the time between measurements of memory usage.
    check_interval: 5s

    # By default limit_mib is set to 80% of ".Values.resources.limits.memory"
    limit_mib: {{ include "opentelemetry-collector.getMemLimitMib" .Values.resources.limits.memory }}

    # By default spike_limit_mib is set to 25% of ".Values.resources.limits.memory"
    spike_limit_mib: {{ include "opentelemetry-collector.getMemSpikeLimitMib" .Values.resources.limits.memory }}

    # By default ballast_size_mib is set to 40% of ".Values.resources.limits.memory"
    ballast_size_mib: {{ include "opentelemetry-collector.getMemBallastSizeMib" .Values.resources.limits.memory }}

service:
  extensions: [health_check]
  pipelines: {}
{{- end }}
