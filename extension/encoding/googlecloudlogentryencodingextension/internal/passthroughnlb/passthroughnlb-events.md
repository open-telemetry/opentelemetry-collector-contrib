# Semantic conventions for GCP Passthrough Network Load Balancer log events

**Status**: Development

This document defines semantic conventions for instrumentations that emit Google Cloud Passthrough Network Load Balancer log events.

## Passthrough Network Load Balancer Log

The encoding format MUST be `gcp.passthrough-nlb`.

Describes GCP Passthrough Network Load Balancer log events for both [External](https://cloud.google.com/load-balancing/docs/network/networklb-monitoring) and [Internal](https://docs.cloud.google.com/load-balancing/docs/internal/internal-logging-monitoring) Network Load Balancers.

**Attributes:**

| Attribute | Stability | Requirement Level | Value Type | Description | Example Values |
| --- | --- | --- | --- | --- | --- |
| [`client.address`](https://opentelemetry.io/docs/specs/semconv/attributes-registry/client/) | ![Stable](https://img.shields.io/badge/-stable-green) | `Recommended` | string | Client IP address of the connection. | `192.168.1.100`; `10.0.0.5` |
| [`client.port`](https://opentelemetry.io/docs/specs/semconv/attributes-registry/client/) | ![Stable](https://img.shields.io/badge/-stable-green) | `Recommended` | int | Client port number of the connection. | `54321`; `8080` |
| [`server.address`](https://opentelemetry.io/docs/specs/semconv/attributes-registry/server/) | ![Stable](https://img.shields.io/badge/-stable-green) | `Recommended` | string | Server IP address of the connection (the load balancer's forwarding rule IP). | `35.192.0.1`; `10.128.0.50` |
| [`server.port`](https://opentelemetry.io/docs/specs/semconv/attributes-registry/server/) | ![Stable](https://img.shields.io/badge/-stable-green) | `Recommended` | int | Server port number of the connection. | `80`; `443` |
| [`network.transport`](https://opentelemetry.io/docs/specs/semconv/attributes-registry/network/) | ![Stable](https://img.shields.io/badge/-stable-green) | `Recommended` | string | Transport layer protocol (translated from IANA protocol number). | `tcp`; `udp`; `icmp` |
| `gcp.load_balancing.passthrough_nlb.packets.start_time` | ![Development](https://img.shields.io/badge/-development-blue) | `Recommended` | string | Start time of the packet sampling window (RFC3339 format). | `2024-01-15T10:30:00.000Z` |
| `gcp.load_balancing.passthrough_nlb.packets.end_time` | ![Development](https://img.shields.io/badge/-development-blue) | `Recommended` | string | End time of the packet sampling window (RFC3339 format). | `2024-01-15T10:31:00.000Z` |
| `gcp.load_balancing.passthrough_nlb.bytes_received` | ![Development](https://img.shields.io/badge/-development-blue) | `Recommended` | int | Number of bytes received from the client. | `1024`; `65536` |
| `gcp.load_balancing.passthrough_nlb.bytes_sent` | ![Development](https://img.shields.io/badge/-development-blue) | `Recommended` | int | Number of bytes sent to the client. | `2048`; `131072` |
| `gcp.load_balancing.passthrough_nlb.packets_received` | ![Development](https://img.shields.io/badge/-development-blue) | `Recommended` | int | Number of packets received from the client. | `10`; `100` |
| `gcp.load_balancing.passthrough_nlb.packets_sent` | ![Development](https://img.shields.io/badge/-development-blue) | `Recommended` | int | Number of packets sent to the client. | `15`; `150` |
| `gcp.load_balancing.passthrough_nlb.rtt` | ![Development](https://img.shields.io/badge/-development-blue) | `Opt-In` | double | Round-trip time in seconds for the connection. | `0.005`; `0.025` |

**Notes:**

- Attributes prefixed with `client.*`, `server.*`, and `network.*` follow the [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/).
- GCP-specific attributes use the `gcp.load_balancing.passthrough_nlb.*` namespace.
- **Protocol translation**: The numeric protocol field from GCP is automatically translated to human-readable names using [IANA Protocol Numbers](https://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml) (e.g., `6` → `tcp`, `17` → `udp`, `1` → `icmp`).
- **Supported log types**: This parser handles both External and Internal Network Load Balancer logs:
  - `type.googleapis.com/google.cloud.loadbalancing.type.ExternalNetworkLoadBalancerLogEntry`
  - `type.googleapis.com/google.cloud.loadbalancing.type.InternalNetworkLoadBalancerLogEntry`

