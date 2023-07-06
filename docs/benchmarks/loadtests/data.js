window.BENCHMARK_DATA = {
  "lastUpdate": 1688669512721,
  "repoUrl": "https://github.com/open-telemetry/opentelemetry-collector-contrib",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "aboten@lightstep.com",
            "name": "Alex Boten",
            "username": "codeboten"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "e4c300367ab7708a1403ad7e2598bae335453840",
          "message": "[chore] update token to use default token (#24019)\n\nSigned-off-by: Alex Boten <aboten@lightstep.com>",
          "timestamp": "2023-07-06T11:42:54-07:00",
          "tree_id": "3185889f4af832d5ff3e2a5cc67f6be4f64311ec",
          "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/commit/e4c300367ab7708a1403ad7e2598bae335453840"
        },
        "date": 1688669471577,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "cpu_percentage_avg",
            "value": 10.465857766799385,
            "unit": "%",
            "extra": "Log10kDPS/OTLP - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 11.997617665053896,
            "unit": "%",
            "extra": "Log10kDPS/OTLP - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 54,
            "unit": "MiB",
            "extra": "Log10kDPS/OTLP - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 78,
            "unit": "MiB",
            "extra": "Log10kDPS/OTLP - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/OTLP - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 8.599366747219063,
            "unit": "%",
            "extra": "Log10kDPS/OTLP-HTTP - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 10.331019859844693,
            "unit": "%",
            "extra": "Log10kDPS/OTLP-HTTP - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 53,
            "unit": "MiB",
            "extra": "Log10kDPS/OTLP-HTTP - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 76,
            "unit": "MiB",
            "extra": "Log10kDPS/OTLP-HTTP - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/OTLP-HTTP - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 12.999430530480238,
            "unit": "%",
            "extra": "Log10kDPS/filelog - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 14.330819798302569,
            "unit": "%",
            "extra": "Log10kDPS/filelog - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 53,
            "unit": "MiB",
            "extra": "Log10kDPS/filelog - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 77,
            "unit": "MiB",
            "extra": "Log10kDPS/filelog - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/filelog - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 13.065979829047123,
            "unit": "%",
            "extra": "Log10kDPS/filelog_checkpoints - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 14.998431713985212,
            "unit": "%",
            "extra": "Log10kDPS/filelog_checkpoints - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 56,
            "unit": "MiB",
            "extra": "Log10kDPS/filelog_checkpoints - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 79,
            "unit": "MiB",
            "extra": "Log10kDPS/filelog_checkpoints - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/filelog_checkpoints - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 36.06534991112398,
            "unit": "%",
            "extra": "Log10kDPS/kubernetes_containers - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 36.988605844912506,
            "unit": "%",
            "extra": "Log10kDPS/kubernetes_containers - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 57,
            "unit": "MiB",
            "extra": "Log10kDPS/kubernetes_containers - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 83,
            "unit": "MiB",
            "extra": "Log10kDPS/kubernetes_containers - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/kubernetes_containers - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 33.13113411294709,
            "unit": "%",
            "extra": "Log10kDPS/k8s_CRI-Containerd - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 35.32445407258429,
            "unit": "%",
            "extra": "Log10kDPS/k8s_CRI-Containerd - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 58,
            "unit": "MiB",
            "extra": "Log10kDPS/k8s_CRI-Containerd - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 86,
            "unit": "MiB",
            "extra": "Log10kDPS/k8s_CRI-Containerd - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/k8s_CRI-Containerd - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 25.26567902790471,
            "unit": "%",
            "extra": "Log10kDPS/k8s_CRI-Containerd_no_attr_ops - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 26.99511042064136,
            "unit": "%",
            "extra": "Log10kDPS/k8s_CRI-Containerd_no_attr_ops - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 55,
            "unit": "MiB",
            "extra": "Log10kDPS/k8s_CRI-Containerd_no_attr_ops - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 83,
            "unit": "MiB",
            "extra": "Log10kDPS/k8s_CRI-Containerd_no_attr_ops - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/k8s_CRI-Containerd_no_attr_ops - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 13.732750364347618,
            "unit": "%",
            "extra": "Log10kDPS/CRI-Containerd - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 15.664021885334119,
            "unit": "%",
            "extra": "Log10kDPS/CRI-Containerd - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 54,
            "unit": "MiB",
            "extra": "Log10kDPS/CRI-Containerd - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 78,
            "unit": "MiB",
            "extra": "Log10kDPS/CRI-Containerd - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/CRI-Containerd - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 14.131877272316833,
            "unit": "%",
            "extra": "Log10kDPS/syslog-tcp-batch-1 - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 15.330150579450931,
            "unit": "%",
            "extra": "Log10kDPS/syslog-tcp-batch-1 - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 52,
            "unit": "MiB",
            "extra": "Log10kDPS/syslog-tcp-batch-1 - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 75,
            "unit": "MiB",
            "extra": "Log10kDPS/syslog-tcp-batch-1 - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/syslog-tcp-batch-1 - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 5.065576899990133,
            "unit": "%",
            "extra": "Log10kDPS/syslog-tcp-batch-100 - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 6.999607585666332,
            "unit": "%",
            "extra": "Log10kDPS/syslog-tcp-batch-100 - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 53,
            "unit": "MiB",
            "extra": "Log10kDPS/syslog-tcp-batch-100 - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 75,
            "unit": "MiB",
            "extra": "Log10kDPS/syslog-tcp-batch-100 - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/syslog-tcp-batch-100 - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 20.79874084422929,
            "unit": "%",
            "extra": "Log10kDPS/FluentForward-SplunkHEC - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 22.327434469801283,
            "unit": "%",
            "extra": "Log10kDPS/FluentForward-SplunkHEC - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 54,
            "unit": "MiB",
            "extra": "Log10kDPS/FluentForward-SplunkHEC - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 76,
            "unit": "MiB",
            "extra": "Log10kDPS/FluentForward-SplunkHEC - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/FluentForward-SplunkHEC - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 14.399119103170744,
            "unit": "%",
            "extra": "Log10kDPS/tcp-batch-1 - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 15.663581551980606,
            "unit": "%",
            "extra": "Log10kDPS/tcp-batch-1 - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 52,
            "unit": "MiB",
            "extra": "Log10kDPS/tcp-batch-1 - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 75,
            "unit": "MiB",
            "extra": "Log10kDPS/tcp-batch-1 - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/tcp-batch-1 - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 5.132963715802308,
            "unit": "%",
            "extra": "Log10kDPS/tcp-batch-100 - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 6.999503445892878,
            "unit": "%",
            "extra": "Log10kDPS/tcp-batch-100 - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 53,
            "unit": "MiB",
            "extra": "Log10kDPS/tcp-batch-100 - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 76,
            "unit": "MiB",
            "extra": "Log10kDPS/tcp-batch-100 - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/tcp-batch-100 - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 0.5332911830648063,
            "unit": "%",
            "extra": "IdleMode - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 2.333030170060915,
            "unit": "%",
            "extra": "IdleMode - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 45,
            "unit": "MiB",
            "extra": "IdleMode - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 64,
            "unit": "MiB",
            "extra": "IdleMode - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "IdleMode - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 41.86222987869726,
            "unit": "%",
            "extra": "Metric10kDPS/OpenCensus - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 43.66382911451321,
            "unit": "%",
            "extra": "Metric10kDPS/OpenCensus - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 54,
            "unit": "MiB",
            "extra": "Metric10kDPS/OpenCensus - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 78,
            "unit": "MiB",
            "extra": "Metric10kDPS/OpenCensus - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Metric10kDPS/OpenCensus - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 25.665364404365764,
            "unit": "%",
            "extra": "Metric10kDPS/OTLP - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 27.652204858482534,
            "unit": "%",
            "extra": "Metric10kDPS/OTLP - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 55,
            "unit": "MiB",
            "extra": "Metric10kDPS/OTLP - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 79,
            "unit": "MiB",
            "extra": "Metric10kDPS/OTLP - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Metric10kDPS/OTLP - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 24.464735382942944,
            "unit": "%",
            "extra": "Metric10kDPS/OTLP-HTTP - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 25.329552070254813,
            "unit": "%",
            "extra": "Metric10kDPS/OTLP-HTTP - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 52,
            "unit": "MiB",
            "extra": "Metric10kDPS/OTLP-HTTP - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 74,
            "unit": "MiB",
            "extra": "Metric10kDPS/OTLP-HTTP - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Metric10kDPS/OTLP-HTTP - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 54.63195238413504,
            "unit": "%",
            "extra": "Metric10kDPS/SignalFx - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 55.838632735611085,
            "unit": "%",
            "extra": "Metric10kDPS/SignalFx - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 56,
            "unit": "MiB",
            "extra": "Metric10kDPS/SignalFx - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 81,
            "unit": "MiB",
            "extra": "Metric10kDPS/SignalFx - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Metric10kDPS/SignalFx - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 40.196685024942234,
            "unit": "%",
            "extra": "MetricsFromFile - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 40.3358373686719,
            "unit": "%",
            "extra": "MetricsFromFile - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 56,
            "unit": "MiB",
            "extra": "MetricsFromFile - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 80,
            "unit": "MiB",
            "extra": "MetricsFromFile - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "MetricsFromFile - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 0,
            "unit": "%",
            "extra": "MetricResourceProcessor/update_and_rename_existing_attributes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 0,
            "unit": "%",
            "extra": "MetricResourceProcessor/update_and_rename_existing_attributes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 0,
            "unit": "MiB",
            "extra": "MetricResourceProcessor/update_and_rename_existing_attributes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 0,
            "unit": "MiB",
            "extra": "MetricResourceProcessor/update_and_rename_existing_attributes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "MetricResourceProcessor/update_and_rename_existing_attributes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 0,
            "unit": "%",
            "extra": "MetricResourceProcessor/set_attribute_on_empty_resource - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 0,
            "unit": "%",
            "extra": "MetricResourceProcessor/set_attribute_on_empty_resource - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 0,
            "unit": "MiB",
            "extra": "MetricResourceProcessor/set_attribute_on_empty_resource - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 0,
            "unit": "MiB",
            "extra": "MetricResourceProcessor/set_attribute_on_empty_resource - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "MetricResourceProcessor/set_attribute_on_empty_resource - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 10.59927402180457,
            "unit": "%",
            "extra": "Trace10kSPS/JaegerGRPC - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 12.665792566546202,
            "unit": "%",
            "extra": "Trace10kSPS/JaegerGRPC - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 52,
            "unit": "MiB",
            "extra": "Trace10kSPS/JaegerGRPC - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 74,
            "unit": "MiB",
            "extra": "Trace10kSPS/JaegerGRPC - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/JaegerGRPC - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 11.066013646446704,
            "unit": "%",
            "extra": "Trace10kSPS/OpenCensus - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 12.32886475274219,
            "unit": "%",
            "extra": "Trace10kSPS/OpenCensus - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 54,
            "unit": "MiB",
            "extra": "Trace10kSPS/OpenCensus - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 79,
            "unit": "MiB",
            "extra": "Trace10kSPS/OpenCensus - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/OpenCensus - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 6.199609812677376,
            "unit": "%",
            "extra": "Trace10kSPS/OTLP-gRPC - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 7.998823621008882,
            "unit": "%",
            "extra": "Trace10kSPS/OTLP-gRPC - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 55,
            "unit": "MiB",
            "extra": "Trace10kSPS/OTLP-gRPC - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 78,
            "unit": "MiB",
            "extra": "Trace10kSPS/OTLP-gRPC - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/OTLP-gRPC - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 9.266210867931548,
            "unit": "%",
            "extra": "Trace10kSPS/OTLP-gRPC-gzip - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 10.665958589229122,
            "unit": "%",
            "extra": "Trace10kSPS/OTLP-gRPC-gzip - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 53,
            "unit": "MiB",
            "extra": "Trace10kSPS/OTLP-gRPC-gzip - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 77,
            "unit": "MiB",
            "extra": "Trace10kSPS/OTLP-gRPC-gzip - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/OTLP-gRPC-gzip - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 5.399707157681717,
            "unit": "%",
            "extra": "Trace10kSPS/OTLP-HTTP - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 7.3323119887112815,
            "unit": "%",
            "extra": "Trace10kSPS/OTLP-HTTP - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 51,
            "unit": "MiB",
            "extra": "Trace10kSPS/OTLP-HTTP - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 71,
            "unit": "MiB",
            "extra": "Trace10kSPS/OTLP-HTTP - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/OTLP-HTTP - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 8.266375712836336,
            "unit": "%",
            "extra": "Trace10kSPS/OTLP-HTTP-gzip - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 9.998965523691862,
            "unit": "%",
            "extra": "Trace10kSPS/OTLP-HTTP-gzip - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 53,
            "unit": "MiB",
            "extra": "Trace10kSPS/OTLP-HTTP-gzip - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 76,
            "unit": "MiB",
            "extra": "Trace10kSPS/OTLP-HTTP-gzip - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/OTLP-HTTP-gzip - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 8.532807534337968,
            "unit": "%",
            "extra": "Trace10kSPS/SAPM - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 9.99785748913365,
            "unit": "%",
            "extra": "Trace10kSPS/SAPM - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 52,
            "unit": "MiB",
            "extra": "Trace10kSPS/SAPM - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 75,
            "unit": "MiB",
            "extra": "Trace10kSPS/SAPM - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/SAPM - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 11.399522674626757,
            "unit": "%",
            "extra": "Trace10kSPS/SAPM-gzip - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 13.331085423490748,
            "unit": "%",
            "extra": "Trace10kSPS/SAPM-gzip - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 64,
            "unit": "MiB",
            "extra": "Trace10kSPS/SAPM-gzip - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 91,
            "unit": "MiB",
            "extra": "Trace10kSPS/SAPM-gzip - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/SAPM-gzip - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 8.466002998630797,
            "unit": "%",
            "extra": "Trace10kSPS/SAPM-zstd - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 10.996811309608606,
            "unit": "%",
            "extra": "Trace10kSPS/SAPM-zstd - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 149,
            "unit": "MiB",
            "extra": "Trace10kSPS/SAPM-zstd - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 240,
            "unit": "MiB",
            "extra": "Trace10kSPS/SAPM-zstd - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/SAPM-zstd - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 25.665394155892102,
            "unit": "%",
            "extra": "Trace10kSPS/Zipkin - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 26.32563060299747,
            "unit": "%",
            "extra": "Trace10kSPS/Zipkin - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 58,
            "unit": "MiB",
            "extra": "Trace10kSPS/Zipkin - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 83,
            "unit": "MiB",
            "extra": "Trace10kSPS/Zipkin - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace10kSPS/Zipkin - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 0,
            "unit": "%",
            "extra": "TraceAttributesProcessor/JaegerGRPC - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 0,
            "unit": "%",
            "extra": "TraceAttributesProcessor/JaegerGRPC - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 0,
            "unit": "MiB",
            "extra": "TraceAttributesProcessor/JaegerGRPC - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 0,
            "unit": "MiB",
            "extra": "TraceAttributesProcessor/JaegerGRPC - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceAttributesProcessor/JaegerGRPC - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 0,
            "unit": "%",
            "extra": "TraceAttributesProcessor/OTLP - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 0,
            "unit": "%",
            "extra": "TraceAttributesProcessor/OTLP - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 0,
            "unit": "MiB",
            "extra": "TraceAttributesProcessor/OTLP - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 0,
            "unit": "MiB",
            "extra": "TraceAttributesProcessor/OTLP - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceAttributesProcessor/OTLP - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 8.532718812044122,
            "unit": "%",
            "extra": "TraceBallast1kSPSWithAttrs/0*0bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 10.664250943666902,
            "unit": "%",
            "extra": "TraceBallast1kSPSWithAttrs/0*0bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 91,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSWithAttrs/0*0bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 146,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSWithAttrs/0*0bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceBallast1kSPSWithAttrs/0*0bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 18.932899307976434,
            "unit": "%",
            "extra": "TraceBallast1kSPSWithAttrs/100*50bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 21.663343228191994,
            "unit": "%",
            "extra": "TraceBallast1kSPSWithAttrs/100*50bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 442,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSWithAttrs/100*50bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 831,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSWithAttrs/100*50bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceBallast1kSPSWithAttrs/100*50bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 15.066036852088516,
            "unit": "%",
            "extra": "TraceBallast1kSPSWithAttrs/10*1000bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 16.658172098335875,
            "unit": "%",
            "extra": "TraceBallast1kSPSWithAttrs/10*1000bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 348,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSWithAttrs/10*1000bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 750,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSWithAttrs/10*1000bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceBallast1kSPSWithAttrs/10*1000bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 37.72564213624982,
            "unit": "%",
            "extra": "TraceBallast1kSPSWithAttrs/20*5000bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 39.9943482786617,
            "unit": "%",
            "extra": "TraceBallast1kSPSWithAttrs/20*5000bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 514,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSWithAttrs/20*5000bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 844,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSWithAttrs/20*5000bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceBallast1kSPSWithAttrs/20*5000bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 8.532969453952804,
            "unit": "%",
            "extra": "TraceBallast1kSPSAddAttrs/0*0bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 9.99764036358648,
            "unit": "%",
            "extra": "TraceBallast1kSPSAddAttrs/0*0bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 95,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSAddAttrs/0*0bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 154,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSAddAttrs/0*0bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceBallast1kSPSAddAttrs/0*0bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 18.99929830398222,
            "unit": "%",
            "extra": "TraceBallast1kSPSAddAttrs/100*50bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 19.994052642412516,
            "unit": "%",
            "extra": "TraceBallast1kSPSAddAttrs/100*50bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 427,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSAddAttrs/100*50bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 833,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSAddAttrs/100*50bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceBallast1kSPSAddAttrs/100*50bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 13.799360797968566,
            "unit": "%",
            "extra": "TraceBallast1kSPSAddAttrs/10*1000bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 15.331436114108522,
            "unit": "%",
            "extra": "TraceBallast1kSPSAddAttrs/10*1000bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 292,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSAddAttrs/10*1000bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 619,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSAddAttrs/10*1000bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceBallast1kSPSAddAttrs/10*1000bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 37.66381718044683,
            "unit": "%",
            "extra": "TraceBallast1kSPSAddAttrs/20*5000bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 39.040713621213456,
            "unit": "%",
            "extra": "TraceBallast1kSPSAddAttrs/20*5000bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 514,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSAddAttrs/20*5000bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 846,
            "unit": "MiB",
            "extra": "TraceBallast1kSPSAddAttrs/20*5000bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "TraceBallast1kSPSAddAttrs/20*5000bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 27.531888878289713,
            "unit": "%",
            "extra": "TraceNoBackend10kSPS/NoMemoryLimit - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 31.66565141310628,
            "unit": "%",
            "extra": "TraceNoBackend10kSPS/NoMemoryLimit - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 63,
            "unit": "MiB",
            "extra": "TraceNoBackend10kSPS/NoMemoryLimit - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 90,
            "unit": "MiB",
            "extra": "TraceNoBackend10kSPS/NoMemoryLimit - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 140100,
            "unit": "spans",
            "extra": "TraceNoBackend10kSPS/NoMemoryLimit - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 31.33249900946088,
            "unit": "%",
            "extra": "TraceNoBackend10kSPS/MemoryLimit - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 34.98903920528541,
            "unit": "%",
            "extra": "TraceNoBackend10kSPS/MemoryLimit - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 54,
            "unit": "MiB",
            "extra": "TraceNoBackend10kSPS/MemoryLimit - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 77,
            "unit": "MiB",
            "extra": "TraceNoBackend10kSPS/MemoryLimit - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 137610,
            "unit": "spans",
            "extra": "TraceNoBackend10kSPS/MemoryLimit - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 9.799338609972752,
            "unit": "%",
            "extra": "Trace1kSPSWithAttrs/0*0bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 11.329145635493393,
            "unit": "%",
            "extra": "Trace1kSPSWithAttrs/0*0bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 54,
            "unit": "MiB",
            "extra": "Trace1kSPSWithAttrs/0*0bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 78,
            "unit": "MiB",
            "extra": "Trace1kSPSWithAttrs/0*0bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace1kSPSWithAttrs/0*0bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 23.931277130880133,
            "unit": "%",
            "extra": "Trace1kSPSWithAttrs/100*50bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 25.326073048878243,
            "unit": "%",
            "extra": "Trace1kSPSWithAttrs/100*50bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 54,
            "unit": "MiB",
            "extra": "Trace1kSPSWithAttrs/100*50bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 76,
            "unit": "MiB",
            "extra": "Trace1kSPSWithAttrs/100*50bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace1kSPSWithAttrs/100*50bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 17.865976827951513,
            "unit": "%",
            "extra": "Trace1kSPSWithAttrs/10*1000bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 19.665986721731095,
            "unit": "%",
            "extra": "Trace1kSPSWithAttrs/10*1000bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 57,
            "unit": "MiB",
            "extra": "Trace1kSPSWithAttrs/10*1000bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 80,
            "unit": "MiB",
            "extra": "Trace1kSPSWithAttrs/10*1000bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace1kSPSWithAttrs/10*1000bytes - Dropped Span Count"
          },
          {
            "name": "cpu_percentage_avg",
            "value": 41.997554367615855,
            "unit": "%",
            "extra": "Trace1kSPSWithAttrs/20*5000bytes - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 42.345369651778775,
            "unit": "%",
            "extra": "Trace1kSPSWithAttrs/20*5000bytes - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 60,
            "unit": "MiB",
            "extra": "Trace1kSPSWithAttrs/20*5000bytes - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 86,
            "unit": "MiB",
            "extra": "Trace1kSPSWithAttrs/20*5000bytes - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Trace1kSPSWithAttrs/20*5000bytes - Dropped Span Count"
          }
        ]
      }
    ]
  }
}