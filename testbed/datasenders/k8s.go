// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// FileLogK8sWriter represents abstract container k8s writer
type FileLogK8sWriter struct {
	file   *os.File
	config string
}

// Ensure FileLogK8sWriter implements LogDataSender.
var _ testbed.LogDataSender = (*FileLogK8sWriter)(nil)

// NewFileLogK8sWriter creates a new data sender that will write kubernetes containerd
// log entries to a file, to be tailed by FileLogReceiver and sent to the collector.
//
// config is an Otelcol config appended to the receivers section after executing fmt.Sprintf on it.
// This implies few things:
//   - it should contain `%s` which will be replaced with the filename
//   - all `%` should be represented as `%%`
//   - indentation style matters. Spaces have to be used for indentation
//     and it should start with two spaces indentation
//
// Example config:
// |`
// |  filelog:
// |    include: [ %s ]
// |    start_at: beginning
// |    operators:
// |      type: regex_parser
// |      regex: ^(?P<log>.*)$
// |  `
func NewFileLogK8sWriter(config string) *FileLogK8sWriter {
	dir, err := os.MkdirTemp("", "namespace-*_test-pod_000011112222333344445555666677778888")
	if err != nil {
		panic("failed to create temp dir")
	}
	dir, err = os.MkdirTemp(dir, "*")
	if err != nil {
		panic("failed to create temp dir")
	}

	file, err := os.CreateTemp(dir, "*.log")
	if err != nil {
		panic("failed to create temp file")
	}

	f := &FileLogK8sWriter{
		file:   file,
		config: config,
	}

	return f
}

func (f *FileLogK8sWriter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (f *FileLogK8sWriter) Start() error {
	return nil
}

func (f *FileLogK8sWriter) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			ills := logs.ResourceLogs().At(i).ScopeLogs().At(j)
			for k := 0; k < ills.LogRecords().Len(); k++ {
				_, err := f.file.Write(append(f.convertLogToTextLine(ills.LogRecords().At(k)), '\n'))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *FileLogK8sWriter) convertLogToTextLine(lr plog.LogRecord) []byte {
	sb := strings.Builder{}

	// Timestamp
	sb.WriteString(time.Unix(0, int64(lr.Timestamp())).Format("2006-01-02T15:04:05.000000000Z"))

	// Severity
	sb.WriteString(" stderr F ")
	sb.WriteString(lr.SeverityText())
	sb.WriteString(" ")

	if lr.Body().Type() == pcommon.ValueTypeStr {
		sb.WriteString(lr.Body().Str())
	}

	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		sb.WriteString(" ")
		sb.WriteString(k)
		sb.WriteString("=")
		switch v.Type() {
		case pcommon.ValueTypeStr:
			sb.WriteString(v.Str())
		case pcommon.ValueTypeInt:
			sb.WriteString(strconv.FormatInt(v.Int(), 10))
		case pcommon.ValueTypeDouble:
			sb.WriteString(strconv.FormatFloat(v.Double(), 'f', -1, 64))
		case pcommon.ValueTypeBool:
			sb.WriteString(strconv.FormatBool(v.Bool()))
		default:
			panic("missing case")
		}
		return true
	})

	return []byte(sb.String())
}

func (f *FileLogK8sWriter) Flush() {
	_ = f.file.Sync()
}

func (f *FileLogK8sWriter) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	// We are testing filelog receiver here.

	return fmt.Sprintf(f.config, f.file.Name())
}

func (f *FileLogK8sWriter) ProtocolName() string {
	return "filelog"
}

func (f *FileLogK8sWriter) GetEndpoint() net.Addr {
	return nil
}

// NewKubernetesContainerWriter returns FileLogK8sWriter with configuration
// to recognize and parse kubernetes container logs
func NewKubernetesContainerWriter() *FileLogK8sWriter {
	return NewFileLogK8sWriter(`
  filelog:
    include: [ %s ]
    start_at: beginning
    include_file_path: true
    include_file_name: false
    operators:
      # Find out which format is used by kubernetes
      - type: router
        id: get-format
        routes:
          - output: parser-docker
            expr: 'body matches "^\\{"'
          - output: parser-crio
            expr: 'body matches "^[^ Z]+ "'
          - output: parser-containerd
            expr: 'body matches "^[^ Z]+Z"'
      # Parse CRI-O format
      - type: regex_parser
        id: parser-crio
        regex: '^(?P<time>[^ Z]+) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) (?P<log>.*)$'
        output: extract_metadata_from_filepath
        timestamp:
          parse_from: body.time
          layout_type: gotime
          layout: '2006-01-02T15:04:05.000000000-07:00'
      # Parse CRI-Containerd format
      - type: regex_parser
        id: parser-containerd
        regex: '^(?P<time>[^ ^Z]+Z) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) (?P<log>.*)$'
        output: extract_metadata_from_filepath
        timestamp:
          parse_from: body.time
          layout: '%%Y-%%m-%%dT%%H:%%M:%%S.%%LZ'
      # Parse Docker format
      - type: json_parser
        id: parser-docker
        output: extract_metadata_from_filepath
        timestamp:
          parse_from: body.time
          layout: '%%Y-%%m-%%dT%%H:%%M:%%S.%%LZ'
      # Extract metadata from file path
      - type: regex_parser
        id: extract_metadata_from_filepath
        regex: '^.*\/(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[a-f0-9\-]{36})\/(?P<container_name>[^\._]+)\/(?P<restart_count>\d+)\.log$'
        parse_from: attributes["log.file.path"]
      # Move out attributes to Attributes
      - type: move
        from: body.stream
        to: attributes["log.iostream"]
      - type: move
        from: body.container_name
        to: attributes["k8s.container.name"]
      - type: move
        from: body.namespace
        to: attributes["k8s.namespace.name"]
      - type: move
        from: body.pod_name
        to: attributes["k8s.pod.name"]
      - type: move
        from: body.restart_count
        to: attributes["k8s.container.restart_count"]
      - type: move
        from: body.uid
        to: attributes["k8s.pod.uid"]
      - type: move
        from: body.log
        to: body
  `)
}

// NewKubernetesCRIContainerdWriter returns FileLogK8sWriter with configuration
// to parse only CRI-Containerd kubernetes logs
func NewKubernetesCRIContainerdWriter() *FileLogK8sWriter {
	return NewFileLogK8sWriter(`
  filelog:
    include: [ %s ]
    start_at: beginning
    include_file_path: true
    include_file_name: false
    operators:
      # Parse CRI-Containerd format
      - type: regex_parser
        id: parser-containerd
        regex: '^(?P<time>[^ ^Z]+Z) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) (?P<log>.*)$'
        output: extract_metadata_from_filepath
        timestamp:
          parse_from: body.time
          layout: '%%Y-%%m-%%dT%%H:%%M:%%S.%%LZ'
      # Extract metadata from file path
      - type: regex_parser
        id: extract_metadata_from_filepath
        regex: '^.*\/(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[a-f0-9\-]{36})\/(?P<container_name>[^\._]+)\/(?P<restart_count>\d+)\.log$'
        parse_from: attributes["log.file.path"]
      # Move out attributes to Attributes
      - type: move
        from: body.stream
        to: attributes["log.iostream"]
      - type: move
        from: body.container_name
        to: attributes["k8s.container.name"]
      - type: move
        from: body.namespace
        to: attributes["k8s.namespace.name"]
      - type: move
        from: body.pod_name
        to: attributes["k8s.pod.name"]
      - type: move
        from: body.restart_count
        to: attributes["k8s.container.restart_count"]
      - type: move
        from: body.uid
        to: attributes["k8s.pod.uid"]
      - type: move
        from: body.log
        to: body
  `)
}

// NewKubernetesCRIContainerdNoAttributesOpsWriter returns FileLogK8sWriter with configuration
// to parse only CRI-Containerd kubernetes logs without reformatting attributes
func NewKubernetesCRIContainerdNoAttributesOpsWriter() *FileLogK8sWriter {
	return NewFileLogK8sWriter(`
  filelog:
    include: [ %s ]
    start_at: beginning
    include_file_path: true
    include_file_name: false
    operators:
      # Parse CRI-Containerd format
      - type: regex_parser
        id: parser-containerd
        regex: '^(?P<time>[^ ^Z]+Z) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) (?P<log>.*)$'
        output: extract_metadata_from_filepath
        timestamp:
          parse_from: body.time
          layout: '%%Y-%%m-%%dT%%H:%%M:%%S.%%LZ'
      # Extract metadata from file path
      - type: regex_parser
        id: extract_metadata_from_filepath
        regex: '^.*\/(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[a-f0-9\-]{36})\/(?P<container_name>[^\._]+)\/(?P<restart_count>\d+)\.log$'
        parse_from: attributes["log.file.path"]
  `)
}

// NewCRIContainerdWriter returns FileLogK8sWriter with configuration
// to parse only CRI-Containerd logs (no extracting metadata from filename)
func NewCRIContainerdWriter() *FileLogK8sWriter {
	return NewFileLogK8sWriter(`
  filelog:
    include: [ %s ]
    start_at: beginning
    include_file_path: true
    include_file_name: false
    operators:
      # Parse CRI-Containerd format
      - type: regex_parser
        id: parser-containerd
        regex: '^(?P<time>[^ ^Z]+Z) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) (?P<log>.*)$'
        timestamp:
          parse_from: body.time
          layout: '%%Y-%%m-%%dT%%H:%%M:%%S.%%LZ'
  `)
}
