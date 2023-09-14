connectors:
    sporadic:
        desicion: 1

exporters:
    splunk_hec:

receivers:
    foo:

pipelines:
    logs:
        receivers: [foo]
        exporters: [connectors]
    logs/splunk_hec:
        receivers: [connectors]
        exporters: [splunk_hec]