{{/*
Default metrics pipeline config
*/}}
{{- define "opentelemetry-collector.metricsPipelineConfig" -}}
receivers:
  # Prometheus receiver scraping the prometheus metrics from the pod itself.
  prometheus:
    config:
      scrape_configs:
      - job_name: 'opentelemetry-collector'
        scrape_interval: 10s
        static_configs:
        - targets: ["${MY_POD_IP}:8888"]
service:
  pipelines:
    metrics:
      receivers: [prometheus]
      processors: [memory_limiter, batch, queued_retry]
{{- end }}
