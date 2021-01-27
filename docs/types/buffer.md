# Buffers

Buffers are used to temporarily store log entries until they can be flushed to their final destination.

There are two types of buffers: `memory` buffers and `disk` buffers.

## Memory Buffers

Memory buffers keep log entries in memory until they are flushed, which makes them very fast. However, because
entries are only stored in memory, they will be lost if the agent is shut down uncleanly. If the agent is shut down
cleanly, they will be saved to the agent's database.

### Memory Buffer Configuration

Memory buffers are configured by setting the `type` field of the `buffer` block on an output to `memory`. 

| Field             | Default          | Description                                                                      |
| ---               | ---              | ---                                                                              |
| `max_entries`     | `1048576` (2^20) | The maximum number of entries stored in the memory buffer                        |
| `max_chunk_size`  | 1000             | The maximum number of entries that are read from the buffer by default           |
| `max_chunk_delay` | 1s               | The maximum amount of time that a reader will wait to batch entries into a chunk |

Example:
```yaml
- type: google_cloud_output
  project_id: my_project_id
  buffer:
    type: memory
    max_entries: 10000
    max_chunk_delay: 1s
    max_chunk_size: 1000
```


## Disk Buffers

Disk buffers store all log entries on disk until they have been successfully flushed to their destination. This means
that, even in the case of an unclean shutdown (kill signal or power loss), no entries will be lost. However, this comes at the cost of
some performance.

By default, a disk buffer can handle roughly 10,000 logs per second. This number is highly subject to the specs of the
machine running the agent, so if exact numbers are important, we'd advise running your own tests.

If you'd like better performance and power loss is not a concern, disabling sync writes improves performance to
(roughly) 100,000 entries per second. This comes at the tradeoff that, if there is a power failure, there may
be logs that are lost or a corruption of the database.

### Disk Buffer Configuration

Disk buffers are configured by setting the `type` field of the `buffer` block on an output to `disk`. Other fields are described below:

| Field             | Default  | Description                                                                                                                              |
| ---               | ---      | ---                                                                                                                                      |
| `max_size`        | `4GiB`   | The maximum size of the disk buffer file in bytes. See [ByteSize](/docs/types/bytesize.md) for details on allowed values.                |
| `max_chunk_size`  | 1000     | The maximum number of entries that are read from the buffer by default                                                                   |
| `max_chunk_delay` | 1s       | The maximum amount of time that a reader will wait to batch entries into a chunk                                                         |
| `path`            | required | The path to the directory which will contain the disk buffer data                                                                        |
| `sync`            | `true`   | Whether to open the database files with the O_SYNC flag. Disabling this improves performance, but relaxes guarantees about log delivery. |

Example:
```yaml
- type: google_cloud_output
  project_id: my_project_id
  buffer:
    type: disk
    max_size: 10000000 # 10MB
    path: /tmp/stanza_buffer
    sync: true
    max_chunk_delay: 1s
    max_chunk_size: 1000
```
