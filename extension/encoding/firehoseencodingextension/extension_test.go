package firehoseencodingextension

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compressAndEncode(content []byte) (string, error) {
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	defer gzipWriter.Close()
	_, err := gzipWriter.Write(content)
	if err != nil {
		return "", err
	}
	gzipWriter.Close()
	encoded := base64.StdEncoding.EncodeToString(compressed.Bytes())
	return encoded, nil
}

func TestUnmarshalLogs_Records(t *testing.T) {
	t.Parallel()

	extension := cloudwatchExtension{
		config: &Config{
			Metrics: Metrics{
				MetricsEncoding: NoEncoding,
			},
			Logs: Logs{
				LogsEncoding: GZipEncoded,
			},
		},
		logger: zap.NewNop(),
	}

	tests := map[string]struct {
		filename string
		err      error
	}{
		"OneLogRecord": {
			filename: "one_log",
		},
		"TwoLogRecords": {
			filename: "two_logs",
		},
		"InvalidRecord": {
			filename: "invalid_log",
			err: fmt.Errorf("failed to add cloudwatch log from record data [0]: %w",
				fmt.Errorf("cloudwatch log from datum [0] is invalid: %w",
					errors.New("cloudwatch log is missing timestamp field"),
				),
			),
		},
		"SomeInvalidRecord": {
			filename: "some_invalid_log",
			err: fmt.Errorf("failed to add cloudwatch log from record data [0]: %w",
				fmt.Errorf("cloudwatch log from datum [1] is invalid: %w",
					errors.New("cloudwatch log is missing timestamp field"),
				),
			),
		},
		"EmptyLogEvents": {
			filename: "empty_log",
			err: fmt.Errorf("failed to add cloudwatch log from record data [0]: %w",
				errors.New("no log records could be obtained from the record"),
			),
		},
	}

	unmarshaller := plog.JSONUnmarshaler{}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata/logs", test.filename+".json"))
			require.NoError(t, err)

			encoded, err := compressAndEncode(content)
			require.NoError(t, err)

			// put the data in the format firehose expects
			// it to be
			p := payload{Records: []record{{Data: encoded}}}
			allData, err := json.Marshal(p)
			require.NoError(t, err)

			result, err := extension.UnmarshalLogs(allData)
			require.Equal(t, test.err, err)
			if err != nil {
				return
			}

			content, err = os.ReadFile(filepath.Join("testdata/logs", test.filename+"_expected.json"))
			require.NoError(t, err)
			expected, err := unmarshaller.UnmarshalLogs(content)
			require.NoError(t, err)

			require.Equal(t, expected, result)
		})
	}
}

func TestUnmarshalLogs_MultipleResources(t *testing.T) {
	t.Parallel()

	extension := cloudwatchExtension{
		config: &Config{
			Metrics: Metrics{
				MetricsEncoding: NoEncoding,
			},
			Logs: Logs{
				LogsEncoding: GZipEncoded,
			},
		},
		logger: zap.NewNop(),
	}

	filename1 := "one_log"
	content1, err := os.ReadFile(filepath.Join("testdata/logs", filename1+".json"))
	require.NoError(t, err)
	encoded1, err := compressAndEncode(content1)
	require.NoError(t, err)

	filename2 := "two_logs"
	content2, err := os.ReadFile(filepath.Join("testdata/logs", filename2+".json"))
	require.NoError(t, err)
	encoded2, err := compressAndEncode(content2)
	require.NoError(t, err)

	// put the data in the format firehose expects it to be
	p := payload{Records: []record{{Data: encoded1}, {Data: encoded2}}}
	allData, err := json.Marshal(p)
	require.NoError(t, err)

	result, err := extension.UnmarshalLogs(allData)
	require.NoError(t, err)

	content, err := os.ReadFile("testdata/logs/multiple_resources_expected.json")
	require.NoError(t, err)
	unmarshaller := plog.JSONUnmarshaler{}
	expected, err := unmarshaller.UnmarshalLogs(content)
	require.NoError(t, err)

	require.Equal(t, expected, result)
}

func TestUnmarshalMetrics(t *testing.T) {
	t.Parallel()

	extension := cloudwatchExtension{
		config: &Config{
			Metrics: Metrics{
				MetricsEncoding: NoEncoding,
			},
			Logs: Logs{
				LogsEncoding: GZipEncoded,
			},
		},
		logger: zap.NewNop(),
	}

	tests := map[string]struct {
		filename  string
		errPrefix string
	}{
		"OneMetric": {
			filename: "one_metric",
		},
		"TwoMetrics": {
			filename: "two_metrics",
		},
		"MultipleResources": {
			filename: "multiple_resources",
		},
		"InvalidRecord": {
			filename:  "invalid_metric",
			errPrefix: "failed to unmarshall metrics from record data [0]:",
		},
		"SomeInvalidRecord": {
			filename:  "some_invalid_metrics",
			errPrefix: "failed to unmarshall metrics from record data [1]:",
		},
		"EmptyRecord": {
			filename:  "empty_record",
			errPrefix: "no resource metrics could be obtained from the record",
		},
	}

	unmarshaller := pmetric.JSONUnmarshaler{}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata/metrics", test.filename+".json"))
			require.NoError(t, err)

			// for each metric, encode base64
			var records []record
			var buf bytes.Buffer
			gjson.ParseBytes(content).ForEach(func(_, value gjson.Result) bool {
				data, e := json.Marshal(value.Value())
				require.NoError(t, e)
				encoded := base64.StdEncoding.EncodeToString(data)
				_, err = buf.WriteString(encoded + "\n")
				require.NoError(t, err)
				records = append(records, record{Data: encoded})
				return true
			})

			p := payload{Records: records}
			allData, err := json.Marshal(p)
			require.NoError(t, err)

			result, err := extension.UnmarshalMetrics(allData)
			if err != nil {
				require.NotEmpty(t, test.errPrefix)
				require.True(t, strings.HasPrefix(err.Error(), test.errPrefix))
				return
			}

			content, err = os.ReadFile(filepath.Join("testdata/metrics", test.filename+"_expected.json"))
			require.NoError(t, err)
			expected, err := unmarshaller.UnmarshalMetrics(content)
			require.NoError(t, err)

			require.Equal(t, expected, result)
		})
	}
}
