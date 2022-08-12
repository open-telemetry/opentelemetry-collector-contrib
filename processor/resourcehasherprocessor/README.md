# Resource Hasher Processor

| Status                   |                       |
| ------------------------ | --------------------- |
| Stability                | [beta]                |
| Supported pipeline types | traces, metrics, logs |
| Distributions            | [contrib]             |

The resource hasher processor reduces the amount of times a set of resource attributes is sent downstream,
replacing all the attributes in resources repeated within a configurable period of time with a hash stored
as in the `lumigo.resource.hash` resource attribute.

## Configuration

```yaml
max_cache_size: <int>                 # Default 10.000
max_cache_entry_age: <time.Duration>  # Default 30s
```

## Behavior under load

The Resource Hasher Processor uses a [Least Recently Used (LRU)](https://en.wikipedia.org/wiki/Cache_replacement_policies#Least_recently_used_(LRU))
cache to store the hashes of resources it knows, so it can be that, under very high cache pressure (with an
amount of different resources larger than the maximum size of the cache), seldom-seen resources might be sent
downstream more often than the `max_cache_entry_age`.

## Hashing algorithm

The Resource Hasher Processor uses a pure-Go implementation of the [xxHash](https://github.com/Cyan4973/xxHash)
algorithm, optimizing speed over anything else, especially cryptographic safety (which is not a concern at all).
