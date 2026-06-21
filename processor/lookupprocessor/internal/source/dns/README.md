# dns Source

Performs DNS lookups supporting forward (A/AAAA) and reverse (PTR) record types. Caching is enabled by default to minimize DNS queries.

## Configuration

| Field | Description | Default |
| ----- | ----------- | ------- |
| `record_type` | DNS record type: `PTR` (IP → hostname), `A` (hostname → IPv4), `AAAA` (hostname → IPv6) | `PTR` |
| `timeout` | Maximum time to wait for DNS query (must be `> 0`) | `1s` |
| `server` | DNS server to use (e.g., `8.8.8.8:53`). Empty uses system resolver | - |
| `multiple_results` | Return all results as comma-separated string instead of just the first | `false` |
| `cache.enabled` | Enable caching | `true` |
| `cache.size` | Maximum cache entries (LRU eviction) | `10000` |
| `cache.ttl` | Time-to-live for successful lookups | `5m` |
| `cache.negative_ttl` | TTL for "not found" entries | `1m` |

## Example

```yaml
processors:
  lookup:
    source:
      type: dns
      record_type: PTR
      cache:
        enabled: true
        size: 10000
        ttl: 5m
        negative_ttl: 1m
    lookups:
      - key: log.attributes["client.ip"]
        attributes:
          - destination: client.hostname
            default: "unknown"
```

## Benchmarks

Run with `go test -bench=. ./internal/source/dns/`

DNS lookup with and without caching (Apple M4 Pro, network latency varies):

| Scenario | ns/op | B/op | allocs/op |
|----------|-------|------|-----------|
| PTR lookup (no cache) | 192,251 | 784 | 16 |
| PTR lookup (cached) | 37 | 0 | 0 |
| PTR lookup (cached, parallel) | 146 | 0 | 0 |

Cache hit is ~5,000x faster than a network lookup, with zero allocations.

## TODO

- [ ] Support TXT, CNAME, MX record lookups
- [ ] Support multiple DNS servers
