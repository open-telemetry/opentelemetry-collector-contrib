# Parsing Cache
Caching is useful when regex is used to parse the same input repeatedly.

The cache is thread safe by leveraging read / write mutex. Reads will lock the mutex with a read lock, while writes will block all operations with a lock.

When the cache is full, the oldest entry is removed to make room for the new entry.

> Note: The cache only works for [regex_parser](../operators/regex_parser.md) as for now.

### Internal Rate Limiting
This process allows improperly configured cache's to perform similar to regex by avoiding the cache entirely.

For example: A kubernetes cluster with 200 active log files will cause a cache of size 10 to "thrash". The cache will avoid locking (and blocking all reads) when eviction rates are too high.

The internal rate limiting is achieved with a new limiter interface.
The limiter tracks number of cache evictions during an interval.
The limiter's count is incremented for every cache eviction.
When the limiter's count reaches the limiter's limit, the throttled() method will return true.
The cache's add() method will skip adding (and locking) when the limiter is throttled.

### `cache` parameters
| Field     | Default   | Description            |
|-----------|-----------|------------------------|
| `size`    | 0         | The size of the cache. |

### Examples
#### Enable cache for regex parser
```yaml
- type: regex_parser
  parse_from: body.message
  regex: '^Host=(?P<host>[^,]+), Type=(?P<type>.*)$'
  cache:
    size: 100
```
