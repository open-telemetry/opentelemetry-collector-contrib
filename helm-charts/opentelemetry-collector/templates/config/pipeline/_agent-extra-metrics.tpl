{{/*
Default config for metrics pipeline running on agent collector deamonset
*/}}
{{- define "opentelemetry-collector.agentMetricsPipelineConfig" -}}
receivers:
  hostmetrics:
    scrapers:
      cpu:
      disk:
      filesystem:
      memory:
      network:
      load:
      processes:
      # Virtual memory and system processes metrics disabled by default
      # swap:
      # process:
processors:
  resourcedetection:
    detectors: [env]

service:
  pipelines:
    metrics:
      receivers: [prometheus, hostmetrics]
      processors: [memory_limiter, resourcedetection, batch, queued_retry]
{{- end }}
