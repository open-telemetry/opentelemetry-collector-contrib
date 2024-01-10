> Deprecated: [v0.92.0] This tool is deprecated and will be removed in a future release.
> See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30187

# Config Metadata YAML Generator (alpha)

This CLI application creates a configuration metadata YAML file for each
Collector component where each file describes the field names, types, default
values, and inline documentation for the component's configuration.

## Operation

By default, this application creates a new `cfg-metadata` output directory
(overridable via the `-o` flag), subdirectories for each component group
(e.g. `receiver`, `exporter`, etc.), and config metadata YAML files within
those directories for each component.

### Command line flags

* `-o <directory>` the name of the default parent directory to be created (defaults to `cfg-metadata`)
* `-s <directory>` the path to the collector source root directory (defaults to `../..`)

## Example Output

The following is an example config metadata YAML file (for the File Exporter):

```yaml
type: '*fileexporter.Config'
doc: |
  Config defines configuration for file exporter.
fields:
- name: path
  kind: string
  default: ""
  doc: |
    Path of the file to write to. Path is relative to current directory.
- name: rotation
  type: '*fileexporter.Rotation'
  kind: ptr
  doc: |
    Rotation defines an option about rotation of telemetry files
  fields:
  - name: max_megabytes
    kind: int
    doc: |
      MaxMegabytes is the maximum size in megabytes of the file before it gets
      rotated. It defaults to 100 megabytes.
  - name: max_days
    kind: int
    doc: |
      MaxDays is the maximum number of days to retain old log files based on the
      timestamp encoded in their filename.  Note that a day is defined as 24
      hours and may not exactly correspond to calendar days due to daylight
      savings, leap seconds, etc. The default is not to remove old log files
      based on age.
  - name: max_backups
    kind: int
    doc: |
      MaxBackups is the maximum number of old log files to retain. The default
      is to 100 files.
  - name: localtime
    kind: bool
    default: false
    doc: |
      LocalTime determines if the time used for formatting the timestamps in
      backup files is the computer's local time.  The default is to use UTC
      time.
- name: format
  kind: string
  default: json
  doc: |
    FormatType define the data format of encoded telemetry data
    Options:
    - json[default]:  OTLP json bytes.
    - proto:  OTLP binary protobuf bytes.
- name: compression
  kind: string
  default: ""
  doc: |
    Compression Codec used to export telemetry data
    Supported compression algorithms:`zstd`

```