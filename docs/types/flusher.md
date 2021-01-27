# Flushers

Flushers handle reading entries from buffers in chunks, flushing them to their final destination, and retrying on failure.

In most cases, the default options will work well, but they may be need tuning for optimal performance or for reducing load
on the destination API.

For example, if you hit an API limit on the number of requests per second, consider decreasing `max_concurrent` and
increasing `max_chunk_entries`. This will make fewer, larger requests which should increase efficiency at the cost of
some latency.

Or, if you have low load and don't care about the higher latency, consider increasing `max_wait` so that entries are sent
less often in larger requests.

## Flusher configuration

Flushers are configured with the `flusher` block on output plugins.

| Field               | Default | Description                                                                                                                                   |
| ---                 | ---     | ---                                                                                                                                           |
| `max_concurrent`    | `16`    | The maximum number of goroutines flushing entries concurrently                                                                                |
