# File Storage

> :construction: This extension is in alpha. Configuration and functionality are subject to change.

The File Storage extension can persist state to the local file system. 

The extension requires read and write access to a directory. A default directory can be used, but it must already exist in order for the extension to operate.

`directory` is the relative or absolute path to the dedicated data storage directory. 
The default directory is `%ProgramData%\Otelcol\FileStorage` on Windows and `/var/lib/otelcol/file_storage` otherwise.

`timeout` is the maximum time to wait for a file lock. This value does not need to be modified in most circumstances.
The default timeout is `1s`.

`compaction` defines how and when files should be compacted.
For now only compaction on start of the collector is supported, and can be enabled by `compaction.on_start` option.

`compaction.directory` is the directory used for compaction (as midstep).

`compaction.max_transaction_size` defines maximum size of the compaction transaction.
A value of zero will ignore transaction sizes.

```
extensions:
  file_storage:
  file_storage/all_settings:
    directory: /var/lib/otelcol/mydir
    timeout: 1s
    compaction:
      on_start: true
      directory: /tmp/
      max_transaction_size: 65_536

service:
  extensions: [file_storage, file_storage/all_settings]
  pipelines:
    traces:
      receivers: [nop]
      processors: [nop]
      exporters: [nop]

# Data pipeline is required to load the config.
receivers:
  nop:
processors:
  nop:
exporters:
  nop:
```
