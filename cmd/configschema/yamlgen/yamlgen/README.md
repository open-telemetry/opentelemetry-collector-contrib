# Config Metadata YAML Generator (alpha)

This CLI application creates config metadata YAML files for Collector components
where each YAML file describes the configuration fields of its corresponding
component.

## Operation

There are two major modes of operation: one that create YAML files in each
Collector component directory in a Collector source tree, and another that
creates YAML files and directories in a new directory.

### Create YAML files in a Collector source tree

This mode of operation creates a `cfg-metadata.yaml` file per component,
writing each one to its corresponding component directory in the locally
checked-out source tree.

```yaml
cd cmd/configschema
go run ./yamlgen
```

### Create YAML files and directories in a new directory

This mode of operation, specified by `-o <dirname>`, creates YAML metadata
files and directories in a new directory created at `dirname`.

```yaml
cd cmd/configschema/yamlgen
go run . -s ../../.. -o cfg-metadata
```

### Command line arguments

* `-o <directory>` create YAML files and directories in a new directory. If not specified, creates YAML files in a Collector source tree.
* `-s <directory>` the path to the collector source root directory (defaults to `../..`)

## Example Output

The following is an example `cfg-metadata.yaml` produced for the File Exporter. 

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