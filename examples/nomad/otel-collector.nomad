job "otel-collector" {
  region = "global"

  datacenters = ["dc1"]
  namespace   = "default"

  type = "service"

  
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "otel-collector" {
    count = 1
    network {
      mode = "bridge"
      port "healthcheck" {
        to = 13133
      }
      port "jaeger-grpc" {
        to = 14250
      }
      port "jaeger-thrift-http" {
        to = 14268
      }
      port "metrics" {
        to = 8888
      }
      port "otlp" {
        to = 4317
      }
      port "otlphttp" {
        to = 4318
      }
      port "zipkin" {
        to = 9411
      }
      port "zpages" {
        to = 55679
      }
    }

    
    vault {
      policies      = [
  "otel"
]
    }
    

    task "otel-collector" {
      driver = "docker"

      config {
        image = "otel/opentelemetry-collector-contrib:0.50.0"
        force_pull = true
        entrypoint = [
          "/otelcol-contrib",
          "--config=local/otel/config.yaml",
        ]

        ports = [
  "otlphttp",
  "zipkin",
  "zpages",
  "healthcheck",
  "jaeger-grpc",
  "jaeger-thrift-http",
  "metrics",
  "otlp"
]

        

      }

      env {
        DATADOG_SERVICE_NAME = "my-dd-service"
        DATADOG_TAG_NAME = "env:local_dev_env"
        HONEYCOMB_DATASET = "my-hny-dataset"
        HOST_DEV = "/hostfs/dev"
        HOST_ETC = "/hostfs/etc"
        HOST_PROC = "/hostfs/proc"
        HOST_RUN = "/hostfs/run"
        HOST_SYS = "/hostfs/sys"
        HOST_VAR = "/hostfs/var"
    }

      template {
        data = <<EOH
---
receivers:
  otlp:
    protocols:
      grpc:
      http:

processors:
  batch:
    timeout: 10s
  memory_limiter:
    limit_mib: 1536
    spike_limit_mib: 512
    check_interval: 5s

exporters:
  logging:
    logLevel: debug

  otlp/hc:
    endpoint: "api.honeycomb.io:443"
    headers: 
      "x-honeycomb-team": "{{ with secret "kv/data/otel/o11y/honeycomb" }}{{ .Data.data.api_key }}{{ end }}"
      "x-honeycomb-dataset": "$HONEYCOMB_DATASET"
  otlp/ls:
    endpoint: ingest.lightstep.com:443
    headers: 
      "lightstep-access-token": "{{ with secret "kv/data/otel/o11y/lightstep" }}{{ .Data.data.api_key }}{{ end }}"

  datadog:
    service: $DATADOG_SERVICE_NAME
    tags:
      - $DATADOG_TAG_NAME
    api:
      key: "{{ with secret "kv/data/otel/o11y/datadog" }}{{ .Data.data.api_key }}{{ end }}"
      site: datadoghq.com

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [logging, otlp/ls, otlp/hc, datadog]

EOH

        change_mode   = "restart"
        destination   = "local/otel/config.yaml"
      }

      

      resources {
        cpu    = 256
        memory = 512
      }
      service {
        name = "opentelemetry-collector"
        port = "metrics"
        tags = ["prometheus"]
      }
      service {
        name = "opentelemetry-collector"
        port = "zipkin"
        tags = ["zipkin"]
      }
      service {
        name = "opentelemetry-collector"
        port = "healthcheck"
        tags = ["health"]
        check {
          type     = "http"
          path     = "/"
          interval = "15s"
          timeout  = "3s"
        }
      }
      service {
        name = "opentelemetry-collector"
        port = "jaeger-grpc"
        tags = ["jaeger-grpc"]
      }
      service {
        name = "opentelemetry-collector"
        port = "jaeger-thrift-http"
        tags = ["jaeger-thrift-http"]
      }
      service {
        name = "opentelemetry-collector"
        port = "zpages"
        tags = ["zpages"]
      }

      
      service {
        tags = [
          "traefik.tcp.routers.otel-collector-grpc.rule=HostSNI(`*`)",
          "traefik.tcp.routers.otel-collector-grpc.entrypoints=grpc",
          "traefik.enable=true",
        ]        
        port = "otlp"
      }

      service {
        tags = [
          "traefik.http.routers.otel-collector-http.rule=Host(`otel-collector-http.localhost`)",
          "traefik.http.routers.otel-collector-http.entrypoints=web",
          "traefik.http.routers.otel-collector-http.tls=false",
          "traefik.enable=true",
        ]
        port = "otlphttp"
      }

    }
  }
}

