// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestParser(t *testing.T) *Parser {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	return op.(*Parser)
}

func TestConfigBuild(t *testing.T) {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	require.IsType(t, &Parser{}, op)
}

func TestConfigBuildFailure(t *testing.T) {
	config := NewConfigWithID("test")
	config.OnError = "invalid_on_error"
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.ErrorContains(t, err, "invalid `on_error` field")
}

func TestConfigBuildFormatError(t *testing.T) {
	config := NewConfigWithID("test")
	config.Format = "invalid_runtime"
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.ErrorContains(t, err, "invalid `format` field")
}

func TestDockerParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parseDocker([]int{})
	require.ErrorContains(t, err, "type '[]int' cannot be parsed as docker container logs")
}

func TestCrioParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parseCRIO([]int{})
	require.ErrorContains(t, err, "type '[]int' cannot be parsed as cri-o container logs")
}

func TestContainerdParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parseContainerd([]int{})
	require.ErrorContains(t, err, "type '[]int' cannot be parsed as containerd logs")
}

func TestFormatDetectionFailure(t *testing.T) {
	parser := newTestParser(t)
	e := &entry.Entry{
		Body: `invalid container format`,
	}
	_, err := parser.detectFormat(e)
	require.ErrorContains(t, err, "entry cannot be parsed as container logs")
}

func TestInternalRecombineCfg(t *testing.T) {
	cfg := createRecombineConfig(Config{MaxLogSize: 102400})
	expected := recombine.NewConfigWithID(recombineInternalID)
	expected.IsLastEntry = "attributes.logtag == 'F'"
	expected.CombineField = entry.NewBodyField()
	expected.CombineWith = ""
	expected.SourceIdentifier = entry.NewAttributeField(attrs.LogFilePath)
	expected.MaxLogSize = 102400
	require.Equal(t, expected, cfg)
}

func TestProcess(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		cases := []struct {
			name   string
			op     func() (operator.Operator, error)
			input  *entry.Entry
			expect *entry.Entry
		}{
			{
				"docker",
				func() (operator.Operator, error) {
					cfg := NewConfigWithID("test_id")
					cfg.AddMetadataFromFilePath = false
					cfg.Format = "docker"
					set := componenttest.NewNopTelemetrySettings()
					return cfg.Build(set)
				},
				&entry.Entry{
					Body: `{"log":"INFO: log line here","stream":"stdout","time":"2029-03-30T08:31:20.545192187Z"}`,
				},
				&entry.Entry{
					Attributes: map[string]any{
						"log.iostream": "stdout",
					},
					Body:      "INFO: log line here",
					Timestamp: time.Date(2029, time.March, 30, 8, 31, 20, 545192187, time.UTC),
				},
			},
			{
				"docker_with_auto_detection",
				func() (operator.Operator, error) {
					cfg := NewConfigWithID("test_id")
					cfg.AddMetadataFromFilePath = false
					set := componenttest.NewNopTelemetrySettings()
					return cfg.Build(set)
				},
				&entry.Entry{
					Body: `{"log":"INFO: log line here","stream":"stdout","time":"2029-03-30T08:31:20.545192187Z"}`,
				},
				&entry.Entry{
					Attributes: map[string]any{
						"log.iostream": "stdout",
					},
					Body:      "INFO: log line here",
					Timestamp: time.Date(2029, time.March, 30, 8, 31, 20, 545192187, time.UTC),
				},
			},
			{
				"docker_with_auto_detection_and_metadata_from_file_path",
				func() (operator.Operator, error) {
					cfg := NewConfigWithID("test_id")
					cfg.AddMetadataFromFilePath = true
					set := componenttest.NewNopTelemetrySettings()
					return cfg.Build(set)
				},
				&entry.Entry{
					Body: `{"log":"INFO: log line here","stream":"stdout","time":"2029-03-30T08:31:20.545192187Z"}`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				&entry.Entry{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Body: "INFO: log line here",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2029, time.March, 30, 8, 31, 20, 545192187, time.UTC),
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				op, err := tc.op()
				require.NoError(t, err, "did not expect operator function to return an error, this is a bug with the test case")

				err = op.Process(t.Context(), tc.input)
				require.NoError(t, err)
				require.Equal(t, tc.expect, tc.input)
				// Stop the operator
				require.NoError(t, op.Stop())
			})
		}
	})

	t.Run("Failure", func(t *testing.T) {
		cases := []struct {
			name           string
			op             func() (operator.Operator, error)
			input          *entry.Entry
			expectedErrMsg string
		}{
			{
				"docker_with_add_metadata_from_filepath_but_not_included",
				func() (operator.Operator, error) {
					cfg := NewConfigWithID("test_id")
					cfg.AddMetadataFromFilePath = true
					cfg.Format = "docker"
					set := componenttest.NewNopTelemetrySettings()
					return cfg.Build(set)
				},
				&entry.Entry{
					Body: `{"log":"INFO: log line here","stream":"stdout","time":"2029-03-30T08:31:20.545192187Z"}`,
				},
				"operator 'test_id' has 'add_metadata_from_filepath' enabled, but the log record attribute 'log.file.path' is missing. Perhaps enable the 'include_file_path' option?",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				op, err := tc.op()
				require.NoError(t, err)

				err = op.Process(t.Context(), tc.input)
				require.ErrorContains(t, err, tc.expectedErrMsg)
				require.NoError(t, op.Stop())
			})
		}
	})
}

func TestRecombineProcess(t *testing.T) {
	cases := []struct {
		name           string
		op             func() (operator.Operator, error)
		input          []*entry.Entry
		expectedOutput []*entry.Entry
	}{
		{
			"crio_standalone_with_auto_detection_and_metadata_from_file_path",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout F standalone crio line which is awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "F",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Body: "standalone crio line which is awesome!",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.FixedZone("", -10*60*60)),
				},
			},
		},
		{
			"crio_standalone_with_auto_detection_and_metadata_from_rotated_file_path",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout F standalone crio line which is awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log.20250219-233547",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "F",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log.20250219-233547",
					},
					Body: "standalone crio line which is awesome!",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.FixedZone("", -10*60*60)),
				},
			},
		},
		{
			"containerd_standalone_with_auto_detection_and_metadata_from_file_path",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F standalone containerd line which is awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "F",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Body:      "standalone containerd line which is awesome!",
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.UTC),
				},
			},
		},
		{
			"containerd_standalone_with_auto_detection_and_metadata_from_rotated_file_path",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F standalone containerd line which is awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log.20250219-233547",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "F",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log.20250219-233547",
					},
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Body:      "standalone containerd line which is awesome!",
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.UTC),
				},
			},
		},
		{
			"crio_multiple_with_auto_detection_and_metadata_from_file_path",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout P standalone crio line which i`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout F s awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "P",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Body:      "standalone crio line which is awesome!",
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.FixedZone("", -10*60*60)),
				},
			},
		},
		{
			"containerd_multiple_with_auto_detection_and_metadata_from_file_path",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout P standalone containerd line which i`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F s awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "P",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Body: "standalone containerd line which is awesome!",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.UTC),
				},
			},
		},
		{
			"containerd_multiple_with_auto_detection_and_metadata_from_file_path_windows",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout P standalone containerd line which i`,
					Attributes: map[string]any{
						attrs.LogFilePath: "C:\\var\\log\\pods\\some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3\\kube-scheduler44\\1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F s awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "C:\\var\\log\\pods\\some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3\\kube-scheduler44\\1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "P",
						attrs.LogFilePath: "C:\\var\\log\\pods\\some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3\\kube-scheduler44\\1.log",
					},
					Body: "standalone containerd line which is awesome!",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.UTC),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			op, err := tc.op()
			require.NoError(t, err)
			defer func() { require.NoError(t, op.Stop()) }()
			r := op.(*Parser)

			fake := testutil.NewFakeOutput(t)
			r.OutputOperators = ([]operator.Operator{fake})

			for _, e := range tc.input {
				require.NoError(t, r.Process(ctx, e))
			}

			fake.ExpectEntries(t, tc.expectedOutput)

			select {
			case e := <-fake.Received:
				require.FailNow(t, "Received unexpected entry: ", "%+v", e)
			default:
			}
		})
	}
}

func TestProcessWithDockerTime(t *testing.T) {
	cases := []struct {
		name           string
		op             func() (operator.Operator, error)
		input          *entry.Entry
		expectedOutput *entry.Entry
	}{
		{
			"docker",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			&entry.Entry{
				Body: `{"log":"INFO: log line here","stream":"stdout","time":"2029-03-30T08:31:20.545192187Z"}`,
				Attributes: map[string]any{
					attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
				},
			},
			&entry.Entry{
				Attributes: map[string]any{
					"log.iostream":    "stdout",
					attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
				},
				Body: "INFO: log line here",
				Resource: map[string]any{
					"k8s.pod.name":                "kube-scheduler-kind-control-plane",
					"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
					"k8s.container.name":          "kube-scheduler44",
					"k8s.container.restart_count": "1",
					"k8s.namespace.name":          "some",
				},
				Timestamp: time.Date(2029, time.March, 30, 8, 31, 20, 545192187, time.UTC),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			op, err := tc.op()
			require.NoError(t, err)
			defer func() { require.NoError(t, op.Stop()) }()
			r := op.(*Parser)

			fake := testutil.NewFakeOutput(t)
			r.OutputOperators = ([]operator.Operator{fake})

			require.NoError(t, r.Process(ctx, tc.input))

			fake.ExpectEntry(t, tc.expectedOutput)

			select {
			case e := <-fake.Received:
				require.FailNow(t, "Received unexpected entry: ", "%+v", e)
			default:
			}
		})
	}
}

func TestCRIRecombineProcessWithFailedDownstreamOperator(t *testing.T) {
	cases := []struct {
		name           string
		op             func() (operator.Operator, error)
		input          []*entry.Entry
		expectedOutput []*entry.Entry
	}{
		{
			"crio_multiple",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout P standalone crio line which i`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout F s awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout F standalone crio2 line which is awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "P",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Body:      "standalone crio line which is awesome!",
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.FixedZone("", -10*60*60)),
				},
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "F",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Body:      "standalone crio2 line which is awesome!",
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.FixedZone("", -10*60*60)),
				},
			},
		},
		{
			"containerd_multiple",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout P standalone containerd line which i`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F s awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F standalone containerd2 line which is awesome!`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "P",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Body: "standalone containerd line which is awesome!",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.UTC),
				},
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "F",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Body: "standalone containerd2 line which is awesome!",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.UTC),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			op, err := tc.op()
			require.NoError(t, err)
			defer func() { require.NoError(t, op.Stop()) }()
			r := op.(*Parser)

			fake := testutil.NewFakeOutputWithProcessError(t)
			r.OutputOperators = ([]operator.Operator{fake})

			for _, e := range tc.input {
				require.NoError(t, r.Process(ctx, e))
			}

			fake.ExpectEntries(t, tc.expectedOutput)

			select {
			case e := <-fake.Received:
				require.FailNow(t, "Received unexpected entry: ", "%+v", e)
			default:
			}
		})
	}
}

func TestPathCacheIntegration(t *testing.T) {
	t.Run("CacheEnabled", func(t *testing.T) {
		config := NewConfigWithID("test")
		config.Cache.Size = 128
		set := componenttest.NewNopTelemetrySettings()
		op, err := config.Build(set)
		require.NoError(t, err)
		parser := op.(*Parser)
		defer func() { require.NoError(t, parser.Stop()) }()

		require.NotNil(t, parser.pathCache, "cache should be initialized when size > 0")
		require.Equal(t, uint16(128), parser.pathCache.MaxSize())
	})

	t.Run("CacheDisabled", func(t *testing.T) {
		config := NewConfigWithID("test")
		config.Cache.Size = 0
		set := componenttest.NewNopTelemetrySettings()
		op, err := config.Build(set)
		require.NoError(t, err)
		parser := op.(*Parser)
		defer func() { require.NoError(t, parser.Stop()) }()

		require.Nil(t, parser.pathCache, "cache should be nil when size is 0")
	})

	t.Run("CacheReusesParsedPaths", func(t *testing.T) {
		config := NewConfigWithID("test")
		config.Cache.Size = 128
		config.AddMetadataFromFilePath = true
		set := componenttest.NewNopTelemetrySettings()
		op, err := config.Build(set)
		require.NoError(t, err)
		parser := op.(*Parser)
		defer func() { require.NoError(t, parser.Stop()) }()

		ctx := t.Context()
		logPath := "/var/log/pods/default_my-pod_12345-67890/container-name/0.log"

		// Create multiple entries with the same log path
		entries := []*entry.Entry{
			{
				Body: `{"log":"line 1","stream":"stdout","time":"2024-01-01T00:00:00.000000000Z"}`,
				Attributes: map[string]any{
					attrs.LogFilePath: logPath,
				},
			},
			{
				Body: `{"log":"line 2","stream":"stdout","time":"2024-01-01T00:00:01.000000000Z"}`,
				Attributes: map[string]any{
					attrs.LogFilePath: logPath,
				},
			},
			{
				Body: `{"log":"line 3","stream":"stdout","time":"2024-01-01T00:00:02.000000000Z"}`,
				Attributes: map[string]any{
					attrs.LogFilePath: logPath,
				},
			},
		}

		fake := testutil.NewFakeOutput(t)
		parser.OutputOperators = []operator.Operator{fake}

		// Process all entries
		for _, e := range entries {
			require.NoError(t, parser.Process(ctx, e))
		}

		// Verify all entries have the same metadata extracted from path
		received := fake.Received
		for i := 0; i < len(entries); i++ {
			select {
			case e := <-received:
				require.Equal(t, "my-pod", e.Resource["k8s.pod.name"])
				require.Equal(t, "12345-67890", e.Resource["k8s.pod.uid"])
				require.Equal(t, "container-name", e.Resource["k8s.container.name"])
				require.Equal(t, "0", e.Resource["k8s.container.restart_count"])
				require.Equal(t, "default", e.Resource["k8s.namespace.name"])
			case <-time.After(time.Second):
				require.FailNow(t, "timeout waiting for entry")
			}
		}

		// Verify cache was used (same path should be cached)
		cached := parser.pathCache.Get(logPath)
		require.NotNil(t, cached, "path should be cached after processing")
		parsedValues, ok := cached.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "my-pod", parsedValues["pod_name"])
		require.Equal(t, "container-name", parsedValues["container_name"])
	})

	t.Run("CacheHandlesDifferentPaths", func(t *testing.T) {
		config := NewConfigWithID("test")
		config.Cache.Size = 10
		config.AddMetadataFromFilePath = true
		set := componenttest.NewNopTelemetrySettings()
		op, err := config.Build(set)
		require.NoError(t, err)
		parser := op.(*Parser)
		defer func() { require.NoError(t, parser.Stop()) }()

		ctx := t.Context()
		paths := []string{
			"/var/log/pods/default_pod1_a1b2c3d4-e5f6-7890-abcd-ef1234567890/container1/0.log",
			"/var/log/pods/default_pod2_b2c3d4e5-f6a7-8901-bcde-f12345678901/container2/1.log",
			"/var/log/pods/kube-system_pod3_c3d4e5f6-a7b8-9012-cdef-123456789012/container3/0.log",
		}

		fake := testutil.NewFakeOutput(t)
		parser.OutputOperators = []operator.Operator{fake}

		// Process entries with different paths
		for i, path := range paths {
			e := &entry.Entry{
				Body: `{"log":"line","stream":"stdout","time":"2024-01-01T00:00:00.000000000Z"}`,
				Attributes: map[string]any{
					attrs.LogFilePath: path,
				},
			}
			require.NoError(t, parser.Process(ctx, e))

			// Verify cache entry exists
			cached := parser.pathCache.Get(path)
			require.NotNil(t, cached, "path %d should be cached", i)
		}

		// Verify all paths are in cache
		for _, path := range paths {
			cached := parser.pathCache.Get(path)
			require.NotNil(t, cached, "path %s should still be in cache", path)
		}
	})

	t.Run("CacheFIFOEviction", func(t *testing.T) {
		config := NewConfigWithID("test")
		config.Cache.Size = 3 // Small cache to test eviction
		config.AddMetadataFromFilePath = true
		set := componenttest.NewNopTelemetrySettings()
		op, err := config.Build(set)
		require.NoError(t, err)
		parser := op.(*Parser)
		defer func() { require.NoError(t, parser.Stop()) }()

		ctx := t.Context()
		fake := testutil.NewFakeOutput(t)
		parser.OutputOperators = []operator.Operator{fake}

		// Fill cache to capacity
		uids := []string{
			"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"b2c3d4e5-f6a7-8901-bcde-f12345678901",
			"c3d4e5f6-a7b8-9012-cdef-123456789012",
		}
		for i := 0; i < 3; i++ {
			path := "/var/log/pods/default_pod" + strconv.Itoa(i) + "_" + uids[i] + "/container/0.log"
			e := &entry.Entry{
				Body: `{"log":"line","stream":"stdout","time":"2024-01-01T00:00:00.000000000Z"}`,
				Attributes: map[string]any{
					attrs.LogFilePath: path,
				},
			}
			require.NoError(t, parser.Process(ctx, e))
		}

		// Add one more to trigger eviction
		newPath := "/var/log/pods/default_pod4_d4e5f6a7-b8c9-0123-def0-234567890123/container/0.log"
		e := &entry.Entry{
			Body: `{"log":"line","stream":"stdout","time":"2024-01-01T00:00:00.000000000Z"}`,
			Attributes: map[string]any{
				attrs.LogFilePath: newPath,
			},
		}
		require.NoError(t, parser.Process(ctx, e))

		// First path should be evicted (FIFO)
		firstPath := "/var/log/pods/default_pod0_a1b2c3d4-e5f6-7890-abcd-ef1234567890/container/0.log"
		cached := parser.pathCache.Get(firstPath)
		require.Nil(t, cached, "first path should be evicted")

		// New path should be cached
		cached = parser.pathCache.Get(newPath)
		require.NotNil(t, cached, "new path should be cached")
	})

	t.Run("CacheStop", func(t *testing.T) {
		config := NewConfigWithID("test")
		config.Cache.Size = 128
		set := componenttest.NewNopTelemetrySettings()
		op, err := config.Build(set)
		require.NoError(t, err)
		parser := op.(*Parser)

		require.NotNil(t, parser.pathCache)
		require.NoError(t, parser.Stop())
	})
}

func TestParseCRIOFields(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected map[string]any
		ok       bool
	}{
		{
			name:  "valid_crio_with_log",
			input: "2024-01-01T00:00:00.000000000-10:00 stdout F log message here",
			expected: map[string]any{
				"time":   "2024-01-01T00:00:00.000000000-10:00",
				"stream": "stdout",
				"logtag": "F",
				"log":    "log message here",
			},
			ok: true,
		},
		{
			name:  "valid_crio_stderr",
			input: "2024-01-01T00:00:00.000000000+05:30 stderr P partial line",
			expected: map[string]any{
				"time":   "2024-01-01T00:00:00.000000000+05:30",
				"stream": "stderr",
				"logtag": "P",
				"log":    "partial line",
			},
			ok: true,
		},
		{
			name:  "valid_crio_no_log_body",
			input: "2024-01-01T00:00:00.000000000-10:00 stdout F",
			expected: map[string]any{
				"time":   "2024-01-01T00:00:00.000000000-10:00",
				"stream": "stdout",
				"logtag": "F",
				"log":    "",
			},
			ok: true,
		},
		{
			name:  "valid_crio_with_space_after_logtag",
			input: "2024-01-01T00:00:00.000000000-10:00 stdout F  log with leading space",
			expected: map[string]any{
				"time":   "2024-01-01T00:00:00.000000000-10:00",
				"stream": "stdout",
				"logtag": "F",
				"log":    "log with leading space",
			},
			ok: true,
		},
		{
			name:  "invalid_missing_stream",
			input: "2024-01-01T00:00:00.000000000-10:00",
			ok:    false,
		},
		{
			name:  "invalid_invalid_stream",
			input: "2024-01-01T00:00:00.000000000-10:00 invalid F log",
			ok:    false,
		},
		{
			name:  "invalid_empty",
			input: "",
			ok:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := parseCRIOFields(tc.input)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestParseContainerdFields(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected map[string]any
		ok       bool
	}{
		{
			name:  "valid_containerd_with_log",
			input: "2024-01-01T00:00:00.000000000Z stdout F log message here",
			expected: map[string]any{
				"time":   "2024-01-01T00:00:00.000000000Z",
				"stream": "stdout",
				"logtag": "F",
				"log":    "log message here",
			},
			ok: true,
		},
		{
			name:  "valid_containerd_stderr",
			input: "2024-01-01T00:00:00.000000000Z stderr P partial line",
			expected: map[string]any{
				"time":   "2024-01-01T00:00:00.000000000Z",
				"stream": "stderr",
				"logtag": "P",
				"log":    "partial line",
			},
			ok: true,
		},
		{
			name:  "valid_containerd_no_log_body",
			input: "2024-01-01T00:00:00.000000000Z stdout F",
			expected: map[string]any{
				"time":   "2024-01-01T00:00:00.000000000Z",
				"stream": "stdout",
				"logtag": "F",
				"log":    "",
			},
			ok: true,
		},
		{
			name:  "valid_containerd_with_space_after_logtag",
			input: "2024-01-01T00:00:00.000000000Z stdout F  log with leading space",
			expected: map[string]any{
				"time":   "2024-01-01T00:00:00.000000000Z",
				"stream": "stdout",
				"logtag": "F",
				"log":    "log with leading space",
			},
			ok: true,
		},
		{
			name:  "invalid_no_Z_suffix",
			input: "2024-01-01T00:00:00.000000000 stdout F log",
			ok:    false,
		},
		{
			name:  "invalid_missing_stream",
			input: "2024-01-01T00:00:00.000000000Z",
			ok:    false,
		},
		{
			name:  "invalid_invalid_stream",
			input: "2024-01-01T00:00:00.000000000Z invalid F log",
			ok:    false,
		},
		{
			name:  "invalid_empty",
			input: "",
			ok:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := parseContainerdFields(tc.input)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestParseLogPath(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected map[string]any
		ok       bool
	}{
		{
			name:  "valid_unix_path",
			input: "/var/log/pods/default_my-pod_12345-67890/container-name/0.log",
			expected: map[string]any{
				"namespace":      "default",
				"pod_name":       "my-pod",
				"uid":            "12345-67890",
				"container_name": "container-name",
				"restart_count":  "0",
			},
			ok: true,
		},
		{
			name:  "valid_rotated_log",
			input: "/var/log/pods/default_my-pod_12345-67890/container-name/0.log.20250219-233547",
			expected: map[string]any{
				"namespace":      "default",
				"pod_name":       "my-pod",
				"uid":            "12345-67890",
				"container_name": "container-name",
				"restart_count":  "0",
			},
			ok: true,
		},
		{
			name:  "valid_with_restart_count",
			input: "/var/log/pods/kube-system_coredns_abc123/container/5.log",
			expected: map[string]any{
				"namespace":      "kube-system",
				"pod_name":       "coredns",
				"uid":            "abc123",
				"container_name": "container",
				"restart_count":  "5",
			},
			ok: true,
		},
		{
			name:  "valid_uuid_format",
			input: "/var/log/pods/default_pod_a1b2c3d4-e5f6-7890-abcd-ef1234567890/container/0.log",
			expected: map[string]any{
				"namespace":      "default",
				"pod_name":       "pod",
				"uid":            "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				"container_name": "container",
				"restart_count":  "0",
			},
			ok: true,
		},
		{
			name:  "invalid_too_short",
			input: "/var/log/pods/container/0.log",
			ok:    false,
		},
		{
			name:  "invalid_missing_pod_part",
			input: "/var/log/pods/default/container/0.log",
			ok:    false,
		},
		{
			name:  "invalid_not_enough_underscores",
			input: "/var/log/pods/default_pod/container/0.log",
			ok:    false,
		},
		{
			name:  "invalid_not_log_extension",
			input: "/var/log/pods/default_pod_uid/container/0.txt",
			ok:    false,
		},
		{
			name:  "invalid_non_numeric_restart_count",
			input: "/var/log/pods/default_pod_uid/container/abc.log",
			ok:    false,
		},
		{
			name:  "invalid_empty",
			input: "",
			ok:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := parseLogPath(tc.input)
			require.Equal(t, tc.ok, ok, "parseLogPath(%q) = %v, want %v", tc.input, ok, tc.ok)
			if tc.ok {
				require.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestSplitOnce(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		sep      byte
		expected struct {
			head string
			tail string
			ok   bool
		}
	}{
		{
			name:  "valid_single_separator",
			input: "head tail",
			sep:   ' ',
			expected: struct {
				head string
				tail string
				ok   bool
			}{
				head: "head",
				tail: "tail",
				ok:   true,
			},
		},
		{
			name:  "valid_multiple_separators",
			input: "a b c d",
			sep:   ' ',
			expected: struct {
				head string
				tail string
				ok   bool
			}{
				head: "a",
				tail: "b c d",
				ok:   true,
			},
		},
		{
			name:  "valid_separator_at_start",
			input: " head tail",
			sep:   ' ',
			expected: struct {
				head string
				tail string
				ok   bool
			}{
				head: "",
				tail: "head tail",
				ok:   true,
			},
		},
		{
			name:  "valid_separator_at_end",
			input: "head tail ",
			sep:   ' ',
			expected: struct {
				head string
				tail string
				ok   bool
			}{
				head: "head",
				tail: "tail ",
				ok:   true,
			},
		},
		{
			name:  "invalid_no_separator",
			input: "notail",
			sep:   ' ',
			expected: struct {
				head string
				tail string
				ok   bool
			}{
				head: "",
				tail: "",
				ok:   false,
			},
		},
		{
			name:  "invalid_empty",
			input: "",
			sep:   ' ',
			expected: struct {
				head string
				tail string
				ok   bool
			}{
				head: "",
				tail: "",
				ok:   false,
			},
		},
		{
			name:  "valid_different_separator",
			input: "key:value",
			sep:   ':',
			expected: struct {
				head string
				tail string
				ok   bool
			}{
				head: "key",
				tail: "value",
				ok:   true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			head, tail, ok := splitOnce(tc.input, tc.sep)
			require.Equal(t, tc.expected.ok, ok)
			if tc.expected.ok {
				require.Equal(t, tc.expected.head, head)
				require.Equal(t, tc.expected.tail, tail)
			}
		})
	}
}
