# FoundationDB Receiver

| Status                   |                   |
| ------------------------ |-------------------|
| Stability                | [beta]            |
| Supported pipeline types | traces            |
| Distributions            | none 

Receives trace data from [FoundationDB](http://foundationdb.org).

## Getting Started

FoundationDB is a distributed database designed to handle large volumes of structured data across clusters of commodity servers.
FoundationDB organizes data as a key/value store and employs ACID transactions for all operations.

This receiver runs a UDP server and accepts distributed trace data from FoundationDB. Trace data from FoundationDB is encoded using
the [MessagePack](https://msgpack.org/index.html) binary serialization format.

## Configuration 

- `endpoint` (default `endpoint` = localhost:8889)
- `max_packet_size` (default 65_527) sets max UDP packet size
- `socket_buffer_size` (default 0 - no buffer) sets buffer size of connection socket in bytes
- `format` (default opentelemetry) sets the trace format from FoundationDB. Versions 7.1 and prior of FoundationDB use the now deprecated, opentracing.io format. 

Examples:

```yaml
receivers:
  foundationdb:
    endpoint: "localhost:8889"
    max_packet_size: 65527
    socket_buffer_size: 2097152
    format: opentelemetry
```


