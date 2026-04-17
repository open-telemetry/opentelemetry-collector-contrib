# gNMI receiver

| Status        |           |
| ------------- |-----------|
| Stability     | [alpha]: metrics   |
| Distributions | [] |
| Issues        | [![Open issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aopen%20label%3Areceiver%2Fgnmi%20&label=open&color=orange&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Areceiver%2Fgnmi) [![Closed issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aclosed%20label%3Areceiver%2Fgnmi%20&label=closed&color=blue&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aclosed+is%3Aissue+label%3Areceiver%2Fgnmi) |
| Code coverage | [![codecov](https://codecov.io/github/open-telemetry/opentelemetry-collector-contrib/graph/main/badge.svg?component=receiver_gnmi)](https://app.codecov.io/gh/open-telemetry/opentelemetry-collector-contrib/tree/main/?components%5B0%5D=receiver_gnmi&displayType=list) |
| [Code Owners](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#becoming-a-code-owner)    | [@atoulme](https://www.github.com/atoulme) |

[alpha]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha

The gNMI receiver collects telemetry metrics from network devices using the
[gNMI (gRPC Network Management Interface)](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md)
protocol. It operates in **dial-in** mode — the collector connects to each
configured device and subscribes to YANG paths using the standard OpenConfig
gNMI Subscribe RPC.

This receiver is vendor-agnostic and has been validated with:

- **Cisco IOS-XE** (Catalyst) — port 57400
- **Cisco NX-OS** (Nexus) — port 50051
- **Arista EOS** — port 6030

It is complementary to the
[yanggrpcreceiver](../yanggrpcreceiver/README.md), which handles
Cisco MDT **dial-out** (device → collector).

---

## How it works

```
  gnmireceiver (collector)
        │  gNMI dial-in (one goroutine per target)
        │
        ├──► Cisco IOS-XE  :57400
        ├──► Cisco NX-OS   :50051
        └──► Arista EOS    :6030
              │
              │  gNMI SubscribeResponse (STREAM mode)
              ▼
        gnmireceiver
              │
              ├─ 1. Receive SubscribeResponse notification
              ├─ 2. Extract path elements → metric name + attributes
              ├─ 3. Resolve Counter vs Gauge (2-level strategy)
              └─ 4. Emit OTLP metrics → next pipeline component
```

### Dial-in vs Dial-out

| | gnmireceiver (this) | yanggrpcreceiver |
|---|---|---|
| Direction | **Dial-in** — collector → device | **Dial-out** — device → collector |
| Protocol | gNMI OpenConfig | Cisco MDT GPBKV |
| Subscriptions configured on | **Collector** | Device |
| Reconnect on failure | Collector redials | Device redials |
| Vendors | Cisco + Arista + any gNMI target | Cisco IOS-XE, NX-OS |

### Recursive tree subscription

When you subscribe to a path, the gNMI STREAM mode automatically retrieves the
entire sub-tree below that path.

- **Full branch capture**: subscribing to `/interfaces/interface` retrieves all
  nested leaves (statistics, state, admin-status) without listing every sub-path.
- **Wildcard (`[name=*]`) support**: subscribes to all list instances simultaneously.
  Example: `/interfaces/interface[name=*]/state/counters` monitors every interface.

### Counter vs Gauge resolution

Numeric fields are classified using a two-level strategy:

| Level | Source | Coverage |
|---|---|---|
| 1 | YANG files (optional) | Any module configured via `module_paths` |
| 2 | Structural node analysis | All paths — based on OpenConfig semantics |

**Level 1 — YANG file lookup** uses the same `YANGParser` as the
`yanggrpcreceiver`. If the parser is configured and recognises the path, its
answer is final and level 2 is skipped.

**Level 2 — Structural node analysis** inspects the gNMI path elements
directly, from most to least precise:

| Priority | Path element found | Decision |
|---|---|---|
| P3 | `counters` or `statistics` | **Counter** — guaranteed cumulative by OpenConfig |
| P2 | `state` (without counters/statistics) | **Counter** unless leaf name suggests a rate |
| P1 | `config` | **Gauge** — configuration leaves are never metrics |
| P0 | None of the above | **Gauge** — safe default |

This approach is structural rather than lexical — it does not rely on an
exhaustive list of field names, so it handles new YANG models automatically.

### Reconnection

Each target runs in its own goroutine. If a connection is lost, the receiver
waits for the configured `redial` delay and reconnects automatically. A clean
session close (`io.EOF`) is not treated as an error.

---

## Configuration

### Full configuration reference

```yaml
receivers:
  gnmi:
    targets:
      - address: "10.0.0.1:57400"
        username: admin
        password: cisco123

        # gNMI encoding: "proto" (default), "json", or "json_ietf"
        encoding: proto

        # Delay before reconnecting after a session failure
        redial: 10s

        # gRPC connection timeout
        timeout: 30s

        # TLS — set insecure: true for lab/plain-text environments
        tls:
          insecure: true
          # For production:
          # insecure: false
          # ca_file: /etc/otel/certs/ca.pem
          # cert_file: /etc/otel/certs/client.crt
          # key_file: /etc/otel/certs/client.key

        subscriptions:
          - name: interfaces
            origin: openconfig
            path: /interfaces/interface/state/counters
            subscription_mode: sample
            sample_interval: 30s

          - name: bgp
            origin: openconfig
            path: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state
            subscription_mode: on_change
            heartbeat_interval: 300s
            suppress_redundant: true

    # Optional: YANG files for level-1 Counter/Gauge resolution.
    # If omitted, only level-2 structural analysis is used.
    # module_paths:
    #   - /etc/otelcol-contrib/yang/vendor/cisco/xe/17181/
    #   - /etc/otelcol-contrib/yang/vendor/cisco/nx/10.3-1/
```

### Configuration parameters

#### Top-level

| Parameter | Type | Default | Description |
|---|---|---|---|
| `targets` | list | required | List of gNMI target devices |
| `module_paths` | list | `[]` | Directories containing `.yang` files for level-1 resolution |

#### Per target (`targets[*]`)

| Parameter | Type | Default | Description |
|---|---|---|---|
| `address` | string | required | `host:port` of the gNMI server on the device |
| `username` | string | `""` | Username for gRPC metadata authentication |
| `password` | string | `""` | Password for gRPC metadata authentication |
| `encoding` | string | `proto` | gNMI encoding: `proto`, `json`, or `json_ietf` |
| `redial` | duration | `10s` | Reconnect delay after session failure |
| `timeout` | duration | `30s` | gRPC connection timeout |
| `tls` | object | — | TLS configuration (see [configtls](https://pkg.go.dev/go.opentelemetry.io/collector/config/configtls)) |
| `subscriptions` | list | required | List of gNMI paths to subscribe to |

#### Per subscription (`targets[*].subscriptions[*]`)

| Parameter | Type | Default | Description |
|---|---|---|---|
| `name` | string | `""` | Descriptive name (used for logging) |
| `origin` | string | `""` | YANG origin: `openconfig`, `eos_native`, or `""` for native Cisco |
| `path` | string | required | gNMI path to subscribe to |
| `subscription_mode` | string | required | `sample`, `on_change`, or `target_defined` |
| `sample_interval` | duration | — | Required for `sample` mode |
| `heartbeat_interval` | duration | `0` | Force update even if value unchanged |
| `suppress_redundant` | bool | `false` | Skip duplicate values |

---

## YANG modules setup (optional — improves Counter/Gauge accuracy)

YANG files enable level-1 resolution, which is more precise than structural
analysis for vendor-specific paths.

```bash
# Clone the YangModels repository
git clone --depth 1 https://github.com/YangModels/yang /etc/otelcol-contrib/yang
```

Then configure `module_paths` to point to the version directories that match
your devices:

```yaml
receivers:
  gnmi:
    module_paths:
      - /etc/otelcol-contrib/yang/vendor/cisco/xe/17181/
      # - /etc/otelcol-contrib/yang/vendor/cisco/nx/10.3-1/
```

---

## Device configuration

### Cisco IOS-XE — enable gNMI dial-in

```
configure terminal
gnmi-yang
gnmi-yang server
gnmi-yang secure-server
```

Verify:
```
show gnxi state detail
```

### Cisco NX-OS — enable gNMI dial-in

```
configure terminal
feature grpc
grpc gnmi max-concurrent-calls 16
grpc use-vrf management
```

Verify:
```
show grpc gnmi service statistics
```

### Arista EOS — enable gNMI dial-in

```
configure terminal
management api gnmi
   transport grpc default
   provider eos-native
```

Verify:
```
show management api gnmi
```

---

## Production deployment example

```yaml
receivers:
  gnmi:
    targets:
      # ── Cisco IOS-XE Catalyst ──────────────────────────────────────────────
      - address: "10.1.0.1:57400"
        username: otel
        password: ${env:CISCO_PASSWORD}
        encoding: proto
        redial: 10s
        tls:
          insecure: false
          ca_file: /etc/otel/certs/ca.pem
        subscriptions:
          - name: interfaces
            origin: openconfig
            path: /interfaces/interface[name=*]/state/counters
            subscription_mode: sample
            sample_interval: 30s
          - name: bgp
            origin: openconfig
            path: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state
            subscription_mode: on_change
            heartbeat_interval: 300s

      # ── Cisco NX-OS Nexus ─────────────────────────────────────────────────
      - address: "10.1.0.2:50051"
        username: otel
        password: ${env:CISCO_PASSWORD}
        encoding: json
        redial: 10s
        tls:
          insecure: true
        subscriptions:
          - name: interfaces
            origin: openconfig
            path: /interfaces/interface[name=*]/state/counters
            subscription_mode: sample
            sample_interval: 30s

      # ── Arista EOS ────────────────────────────────────────────────────────
      - address: "10.1.0.3:6030"
        username: otel
        password: ${env:ARISTA_PASSWORD}
        encoding: json_ietf
        redial: 10s
        tls:
          insecure: true
        subscriptions:
          - name: interfaces
            origin: openconfig
            path: /interfaces/interface[name=*]/state/counters
            subscription_mode: sample
            sample_interval: 30s
          - name: bgp
            origin: openconfig
            path: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state
            subscription_mode: on_change
            heartbeat_interval: 300s
          - name: cpu
            origin: eos_native
            path: /Kernel/proc/cpu/utilization/total
            subscription_mode: sample
            sample_interval: 30s

  # Dial-out Cisco MDT (complement to this receiver)
  yanggrpc:
    endpoint: "0.0.0.0:57500"

processors:
  batch:

exporters:
  splunk_hec:
    endpoint: "https://splunk.example.com:8088/services/collector"
    token: ${env:SPLUNK_HEC_TOKEN}
    index: network_metrics

service:
  pipelines:
    metrics:
      receivers: [gnmi, yanggrpc]
      processors: [batch]
      exporters: [splunk_hec]
```

---

## Example OTLP output

### Counter — `openconfig.interfaces.interface.state.counters.in-octets`

Emitted as a monotonic Sum (cumulative counter):

```
openconfig.interfaces.interface.state.counters.in-octets
  {name="Ethernet1", device.address="10.1.0.3", host="10.1.0.3"}  1234567890
```

### Gauge — `openconfig.interfaces.interface.state.counters.in-utilization`

Emitted as a Gauge (instantaneous measurement):

```
openconfig.interfaces.interface.state.counters.in-utilization
  {name="Ethernet1", device.address="10.1.0.3", host="10.1.0.3"}  42
```

### Info — `openconfig.interfaces.interface.state.admin-status_info`

String values are emitted as info metrics:

```
openconfig.interfaces.interface.state.admin-status_info
  {value="UP", name="Ethernet1", device.address="10.1.0.3"}  1.0
```

---

## Resource attributes

Every metric carries these resource attributes:

| Attribute | Example | Description |
|---|---|---|
| `device.address` | `10.1.0.3:6030` | Target address as configured |
| `host` | `10.1.0.3:6030` | Same — used as Splunk host dimension |

---

## Relation to yanggrpcreceiver

Both receivers work together to provide complete Cisco + Arista coverage:

```
Cisco Catalyst / Nexus ──► yanggrpcreceiver  (dial-out, MDT GPBKV)
Cisco IOS-XE / NX-OS   ──► gnmireceiver     (dial-in,  gNMI OpenConfig)
Arista EOS             ──► gnmireceiver     (dial-in,  gNMI OpenConfig)
```

The two receivers share the same `YANGParser` for Counter/Gauge resolution,
ensuring consistent metric types regardless of which protocol delivered the data.

---

## Troubleshooting

### Connection refused

Verify that the gNMI service is enabled on the device and that the port is
reachable from the collector:

```bash
# Cisco IOS-XE
show gnxi state detail

# Cisco NX-OS
show grpc gnmi service statistics

# Arista EOS
show management api gnmi
```

### All metrics appear as Gauge

Level-1 YANG resolution is not active (no `module_paths` configured) and the
paths do not contain a `counters` or `statistics` node for level-2.

Verify the subscription path includes a structural counter container:

```
# Good — structural resolver identifies "counters" node → Counter
/interfaces/interface/state/counters/in-octets

# Ambiguous — no counter container → Gauge unless YANG files are loaded
/interfaces/interface/state/in-octets
```

### Wildcard not returning data

Some older gNMI implementations do not support the `[name=*]` wildcard syntax.

Check the device logs. If `*` is rejected, use a more specific path:

```yaml
# Instead of wildcard
path: /interfaces/interface[name=*]/state/counters

# Use the parent path (device streams all instances)
path: /interfaces/interface/state/counters
```

### Missing sub-branches

Ensure the device supports recursive Subscribe for the requested path. Verify
with a gNMI client tool:

```bash
gnmic -a 10.1.0.3:6030 -u admin -p admin --insecure get \
  --path "/interfaces/interface[name=Ethernet1]/state"
```

If sub-branches are absent in the response, the device may require more specific paths.

### Authentication failures

Cisco and Arista both accept credentials as gRPC metadata headers. Verify the
user has at least read-only access:

```
# Cisco IOS-XE
username otel privilege 1 secret <password>

# Arista EOS
role otel
   10 permit command show.*
username otel role otel secret <password>
```
