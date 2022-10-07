# Config Schema YAML Generator (alpha)

This CLI application creates configschema YAML files for collector components
where each YAML file describes the configuration fields of its corresponding
component.

## Operation

There are two major modes of operation: one that create YAML files in each
collector component directory and another that creates a zip archive comprising
all the collector YAML files.


### Create YAML files

This mode of operation creates a `cfgschema.yaml` file per component,
writing each one to its corresponding component directory in the locally
checked-out source tree.


```yaml
cd cmd/configschema
go run ./yamlgen
```

### Create a Zip archive

This mode of operation, specified by the `-z` flag, creates a zip
archive comprising all the component YAML files.

```yaml
cd cmd/configschema/yamlgen
go run . -d ../../.. -z -f myfile.zip
```

### Command line arguments

* `-z` create a zip archive
* `-f <filename>` write the zip archive to filename (for use with `-z`, defaults to `yamlgen.zip`)
* `-d <directory>` the path to the collector source root directory (defaults to `../..`)

## Example Output

The following is an example `cfgschema.yaml` produced for the File Exporter. 

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