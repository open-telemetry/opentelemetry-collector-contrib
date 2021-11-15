# File Log Exporter

This exporter will write data to files. Each log message should contain at least a `file_name_key` 
that will be used to write files.

Raw logs body are written into the files.
Using the `filelog` receiver, it's possible to recursively mirror a full log directory. 

Supported pipeline types: logs

## Getting Started

The following settings are required:

- `path` (no default): where to write information.
- `basedir_key` (default empty): base dir attribute key from resource or log entry. Base dir will be appended to the `path` when writing files. This can be used to route logs from different sources to different directories (ex : pods, sites...).
- `file_name_key` (default "file.name"): filename attribute key from log entry. If the attribute value is empty, warning will be logged. The attribute value can contain relative or absolute paths. Ex : `/absolute/path/file.log`, `file.log`, `relative/path/file.log`. Absolute path will be treated as relative. Missing attribute will result in skipping the log and logging a warning.
- `max_open_files` (default 1024): maximum number of opened file. When reached, least recently used file will be closed. A new log entry to a closed file will re-open the file. 0 means unlimited.

Example:

```yaml
receivers:
  filelog:
    include: [ ./**/*.* ] # workdir is /opt/my_app/log
    include_file_path: true # Add file.path attribute to message
    attributes:
      pod.name: pod-foo

# Files will be written to ./log/pod-foo/.
# If receiver finds a file under /opt/my_app/log/subdir/file.log, it will be written as ./log/pod-foo/subdir/file.log
exporters:
  filelog:
    path: ./log
    basedir_key: "pod.name"
    file_name_key: "file.path"
    max_open_files: 1024
```
