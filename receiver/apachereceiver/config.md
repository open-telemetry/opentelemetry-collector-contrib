# Apache Reference

MetricsBuilderConfig is a configuration for apache metrics builder.


### apachereceiver-Config

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| collection_interval |[time-Duration](#time-Duration)| 10s |  |
| initial_delay |[time-Duration](#time-Duration)| 1s |  |
| timeout |[time-Duration](#time-Duration)| <no value> |  |
| endpoint |string| http://localhost:8080/server-status?auto |  |
| proxy_url |string|  |  |
| tls |[tls-TLSClientSetting](#tls-TLSClientSetting)| <no value> |  |
| read_buffer_size |int| <no value> |  |
| write_buffer_size |int| <no value> |  |
| timeout |[time-Duration](#time-Duration)| 10s |  |
| headers |map[string]configopaque.String| <no value> |  |
| customroundtripper |func(http.RoundTripper) (http.RoundTripper, error)| <no value> |  |
| auth |[auth-Authentication](#auth-Authentication)| <no value> |  |
| compression |configcompression.CompressionType|  |  |
| max_idle_conns |[max-idle-conns-int](#max-idle-conns-int)| <no value> |  |
| max_idle_conns_per_host |[max-idle-conns-per-host-int](#max-idle-conns-per-host-int)| <no value> |  |
| max_conns_per_host |[max-conns-per-host-int](#max-conns-per-host-int)| <no value> |  |
| idle_conn_timeout |[idle-conn-timeout-Duration](#idle-conn-timeout-Duration)| <no value> |  |
| disable_keep_alives |bool| false |  |
| http2_read_idle_timeout |[time-Duration](#time-Duration)| <no value> |  |
| http2_ping_timeout |[time-Duration](#time-Duration)| <no value> |  |
| metrics |[metrics-MetricsConfig](#metrics-MetricsConfig)| <no value> | MetricsConfig provides config for apache metrics.  |
| resource_attributes |[resource-attributes-ResourceAttributesConfig](#resource-attributes-ResourceAttributesConfig)| <no value> | ResourceAttributesConfig provides config for apache resource attributes.  |

### tls-TLSClientSetting

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| ca_file |string|  |  |
| ca_pem |configopaque.String|  |  |
| cert_file |string|  |  |
| cert_pem |configopaque.String|  |  |
| key_file |string|  |  |
| key_pem |configopaque.String|  |  |
| min_version |string|  |  |
| max_version |string|  |  |
| reload_interval |[time-Duration](#time-Duration)| <no value> |  |
| insecure |bool| false |  |
| insecure_skip_verify |bool| false |  |
| server_name_override |string|  |  |

### auth-Authentication

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| authenticator |[authenticator-ID](#authenticator-ID)| <no value> |  |

### metrics-MetricsConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| apache.cpu.load |[apache-cpu-load-MetricConfig](#apache-cpu-load-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.cpu.time |[apache-cpu-time-MetricConfig](#apache-cpu-time-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.current_connections |[apache-current-connections-MetricConfig](#apache-current-connections-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.load.1 |[apache-load-1-MetricConfig](#apache-load-1-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.load.15 |[apache-load-15-MetricConfig](#apache-load-15-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.load.5 |[apache-load-5-MetricConfig](#apache-load-5-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.request.time |[apache-request-time-MetricConfig](#apache-request-time-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.requests |[apache-requests-MetricConfig](#apache-requests-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.scoreboard |[apache-scoreboard-MetricConfig](#apache-scoreboard-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.traffic |[apache-traffic-MetricConfig](#apache-traffic-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.uptime |[apache-uptime-MetricConfig](#apache-uptime-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |
| apache.workers |[apache-workers-MetricConfig](#apache-workers-MetricConfig)| <no value> | MetricConfig provides common config for a particular metric.  |

### apache-cpu-load-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-cpu-time-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-current-connections-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-load-1-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-load-15-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-load-5-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-request-time-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-requests-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-scoreboard-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-traffic-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-uptime-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-workers-MetricConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### resource-attributes-ResourceAttributesConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| apache.server.name |[apache-server-name-ResourceAttributeConfig](#apache-server-name-ResourceAttributeConfig)| <no value> | ResourceAttributeConfig provides common config for a particular resource attribute.  |
| apache.server.port |[apache-server-port-ResourceAttributeConfig](#apache-server-port-ResourceAttributeConfig)| <no value> | ResourceAttributeConfig provides common config for a particular resource attribute.  |

### apache-server-name-ResourceAttributeConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### apache-server-port-ResourceAttributeConfig

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### time-Duration 
An optionally signed sequence of decimal numbers, each with a unit suffix, such as `300ms`, `-1.5h`, or `2h45m`. Valid time units are `ns`, `us`, `ms`, `s`, `m`, `h`.