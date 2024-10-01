# Redis Receiver Reference

ScraperControllerSettings defines common settings for a scraper controller
configuration. Scraper controller receivers can embed this struct
and extend it with more fields if needed.


### redisreceiver-Config

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| collection_interval |[time-Duration](#time-Duration)| 10s |  |
| endpoint |string|  | Endpoint configures the address for this network connection. For TCP and UDP networks, the address has the form "host:port". The host must be a literal IP address, or a host name that can be resolved to IP addresses. The port must be a literal port number or a service name. If the host is a literal IPv6 address it must be enclosed in square brackets, as in "[2001:db8::1]:80" or "[fe80::1%zone]:80". The zone specifies the scope of the literal IPv6 address as defined in RFC 4007.  |
| transport |string| tcp | Transport to use. Known protocols are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only), "udp", "udp4" (IPv4-only), "udp6" (IPv6-only), "ip", "ip4" (IPv4-only), "ip6" (IPv6-only), "unix", "unixgram" and "unixpacket".  |
| username |string|  | Optional username. Use the specified username to authenticate the current connection with one of the connections defined in the ACL list when connecting to a Redis 6.0 instance, or greater, that is using the Redis ACL system. |
| password |string|  | Optional password. Must match the password specified in the requirepass server configuration option or the user's password when connecting to a Redis 6.0 instance, or greater, that is using the Redis ACL system. |
| tls |[tls-TLSClientSetting](#tls-TLSClientSetting)| <no value> | TLSClientSetting contains TLS configurations that are specific to client connections in addition to the common configurations. This should be used by components configuring TLS client connections.  |
| metrics |[metrics-MetricsSettings](#metrics-MetricsSettings)| <no value> | MetricsSettings provides settings for redisreceiver metrics.  |

### tls-TLSClientSetting

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| ca_file |string|  | Path to the CA cert. For a client this verifies the server certificate. For a server this verifies client certificates. If empty uses system root CA. (optional)  |
| cert_file |string|  | Path to the TLS cert to use for TLS required connections. (optional)  |
| key_file |string|  | Path to the TLS key to use for TLS required connections. (optional)  |
| min_version |string|  | MinVersion sets the minimum TLS version that is acceptable. If not set, refer to crypto/tls for defaults. (optional)  |
| max_version |string|  | MaxVersion sets the maximum TLS version that is acceptable. If not set, refer to crypto/tls for defaults. (optional)  |
| reload_interval |[time-Duration](#time-Duration)| <no value> | ReloadInterval specifies the duration after which the certificate will be reloaded If not set, it will never be reloaded (optional)  |
| insecure |bool| true | In gRPC when set to true, this is used to disable the client transport security. See https://godoc.org/google.golang.org/grpc#WithInsecure. In HTTP, this disables verifying the server's certificate chain and host name (InsecureSkipVerify in the tls Config). Please refer to https://godoc.org/crypto/tls#Config for more information. (optional, default false)  |
| insecure_skip_verify |bool| false | InsecureSkipVerify will enable TLS but not verify the certificate.  |
| server_name_override |string|  | ServerName requested by client for virtual hosting. This sets the ServerName in the TLSConfig. Please refer to https://godoc.org/crypto/tls#Config for more information. (optional)  |

### metrics-MetricsSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| redis.clients.blocked |[redis-clients-blocked-MetricSettings](#redis-clients-blocked-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.clients.connected |[redis-clients-connected-MetricSettings](#redis-clients-connected-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.clients.max_input_buffer |[redis-clients-max-input-buffer-MetricSettings](#redis-clients-max-input-buffer-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.clients.max_output_buffer |[redis-clients-max-output-buffer-MetricSettings](#redis-clients-max-output-buffer-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.cmd.calls |[redis-cmd-calls-MetricSettings](#redis-cmd-calls-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.cmd.usec |[redis-cmd-usec-MetricSettings](#redis-cmd-usec-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.commands |[redis-commands-MetricSettings](#redis-commands-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.commands.processed |[redis-commands-processed-MetricSettings](#redis-commands-processed-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.connections.received |[redis-connections-received-MetricSettings](#redis-connections-received-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.connections.rejected |[redis-connections-rejected-MetricSettings](#redis-connections-rejected-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.cpu.time |[redis-cpu-time-MetricSettings](#redis-cpu-time-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.db.avg_ttl |[redis-db-avg-ttl-MetricSettings](#redis-db-avg-ttl-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.db.expires |[redis-db-expires-MetricSettings](#redis-db-expires-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.db.keys |[redis-db-keys-MetricSettings](#redis-db-keys-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.keys.evicted |[redis-keys-evicted-MetricSettings](#redis-keys-evicted-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.keys.expired |[redis-keys-expired-MetricSettings](#redis-keys-expired-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.keyspace.hits |[redis-keyspace-hits-MetricSettings](#redis-keyspace-hits-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.keyspace.misses |[redis-keyspace-misses-MetricSettings](#redis-keyspace-misses-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.latest_fork |[redis-latest-fork-MetricSettings](#redis-latest-fork-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.maxmemory |[redis-maxmemory-MetricSettings](#redis-maxmemory-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.memory.fragmentation_ratio |[redis-memory-fragmentation-ratio-MetricSettings](#redis-memory-fragmentation-ratio-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.memory.lua |[redis-memory-lua-MetricSettings](#redis-memory-lua-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.memory.peak |[redis-memory-peak-MetricSettings](#redis-memory-peak-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.memory.rss |[redis-memory-rss-MetricSettings](#redis-memory-rss-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.memory.used |[redis-memory-used-MetricSettings](#redis-memory-used-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.net.input |[redis-net-input-MetricSettings](#redis-net-input-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.net.output |[redis-net-output-MetricSettings](#redis-net-output-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.rdb.changes_since_last_save |[redis-rdb-changes-since-last-save-MetricSettings](#redis-rdb-changes-since-last-save-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.replication.backlog_first_byte_offset |[redis-replication-backlog-first-byte-offset-MetricSettings](#redis-replication-backlog-first-byte-offset-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.replication.offset |[redis-replication-offset-MetricSettings](#redis-replication-offset-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.role |[redis-role-MetricSettings](#redis-role-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.slaves.connected |[redis-slaves-connected-MetricSettings](#redis-slaves-connected-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |
| redis.uptime |[redis-uptime-MetricSettings](#redis-uptime-MetricSettings)| <no value> | MetricSettings provides common settings for a particular metric.  |

### redis-clients-blocked-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-clients-connected-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-clients-max-input-buffer-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-clients-max-output-buffer-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-cmd-calls-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| false |  |

### redis-cmd-usec-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| false |  |

### redis-commands-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-commands-processed-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-connections-received-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-connections-rejected-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-cpu-time-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-db-avg-ttl-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-db-expires-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-db-keys-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-keys-evicted-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-keys-expired-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-keyspace-hits-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-keyspace-misses-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-latest-fork-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-maxmemory-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| false |  |

### redis-memory-fragmentation-ratio-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-memory-lua-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-memory-peak-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-memory-rss-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-memory-used-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-net-input-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-net-output-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-rdb-changes-since-last-save-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-replication-backlog-first-byte-offset-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-replication-offset-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-role-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| false |  |

### redis-slaves-connected-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### redis-uptime-MetricSettings

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
| enabled |bool| true |  |

### time-Duration 
An optionally signed sequence of decimal numbers, each with a unit suffix, such as `300ms`, `-1.5h`, or `2h45m`. Valid time units are `ns`, `us`, `ms`, `s`, `m`, `h`.