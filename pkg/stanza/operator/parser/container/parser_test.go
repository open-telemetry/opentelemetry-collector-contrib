// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
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
	expected.MaxBatchSize = 0
	expected.MaxUnmatchedBatchSize = 0
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

func TestProcessWithIfCondition(t *testing.T) {
	cases := []struct {
		name           string
		op             func() (operator.Operator, error)
		input          *entry.Entry
		expectedOutput *entry.Entry
	}{
		{
			"if_condition_false_skips_non_container_log",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = false
				cfg.IfExpr = `attributes["log.file.name"] == "k8s.log"`
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			&entry.Entry{
				Body: `a random non-k8s log`,
				Attributes: map[string]any{
					"log.file.name": "non-k8s.log",
				},
			},
			&entry.Entry{
				Body: `a random non-k8s log`,
				Attributes: map[string]any{
					"log.file.name": "non-k8s.log",
				},
			},
		},
		{
			"if_condition_true_processes_container_log",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = false
				cfg.IfExpr = `attributes["log.file.name"] == "k8s.log"`
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			&entry.Entry{
				Body: `{"log":"INFO: log line here","stream":"stdout","time":"2029-03-30T08:31:20.545192187Z"}`,
				Attributes: map[string]any{
					"log.file.name": "k8s.log",
				},
			},
			&entry.Entry{
				Body: "INFO: log line here",
				Attributes: map[string]any{
					"log.file.name": "k8s.log",
					"log.iostream":  "stdout",
				},
				Timestamp: time.Date(2029, time.March, 30, 8, 31, 20, 545192187, time.UTC),
			},
		},
		{
			"if_condition_false_skips_docker_format_detection",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = false
				cfg.Format = "docker"
				cfg.IfExpr = `attributes["process"] == "true"`
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			&entry.Entry{
				Body: `invalid docker log that would fail parsing`,
				Attributes: map[string]any{
					"process": "false",
				},
			},
			&entry.Entry{
				Body: `invalid docker log that would fail parsing`,
				Attributes: map[string]any{
					"process": "false",
				},
			},
		},
		{
			"if_condition_false_skips_crio_format_detection",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = false
				cfg.IfExpr = `attributes["process"] == "true"`
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			&entry.Entry{
				Body: `invalid crio log that would fail parsing`,
				Attributes: map[string]any{
					"process": "false",
				},
			},
			&entry.Entry{
				Body: `invalid crio log that would fail parsing`,
				Attributes: map[string]any{
					"process": "false",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := tc.op()
			require.NoError(t, err)
			defer func() { require.NoError(t, op.Stop()) }()

			err = op.Process(t.Context(), tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expectedOutput, tc.input)
		})
	}
}

func TestProcessWithOnErrorSendQuiet(t *testing.T) {
	t.Run("on_error_send_quiet_respects_if_condition", func(t *testing.T) {
		// This test verifies that when an 'if' condition filters out an entry,
		// it doesn't attempt format detection (which would fail for non-container logs)
		// and just passes the entry through unchanged
		cfg := NewConfigWithID("test_id")
		cfg.AddMetadataFromFilePath = false
		cfg.OnError = "send_quiet"
		cfg.IfExpr = `attributes["is_container"] == "true"`
		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
		require.NoError(t, err)
		defer func() { require.NoError(t, op.Stop()) }()

		input := &entry.Entry{
			Body: `a random non-container log`,
			Attributes: map[string]any{
				"is_container": "false",
			},
		}

		err = op.Process(t.Context(), input)
		require.NoError(t, err)
		// Entry passes through unchanged because if condition filtered it
		require.Equal(t, &entry.Entry{
			Body: `a random non-container log`,
			Attributes: map[string]any{
				"is_container": "false",
			},
		}, input)
	})
}

// TestDockerProcessBatchDoesNotSplitBatches verifies that the container parser processes
// batches of docker entries without splitting them into individual entries.
func TestDockerProcessBatchDoesNotSplitBatches(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("CanProcess").Return(true)
	output.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

	cfg := NewConfigWithID("test_id")
	cfg.AddMetadataFromFilePath = false
	cfg.Format = "docker"
	cfg.OutputIDs = []string{"test-output"}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	defer func() { require.NoError(t, op.Stop()) }()

	err = op.SetOutputs([]operator.Operator{output})
	require.NoError(t, err)

	ctx := t.Context()

	entry1 := entry.New()
	entry1.Body = `{"log":"INFO: first line","stream":"stdout","time":"2029-03-30T08:31:20.545Z"}`

	entry2 := entry.New()
	entry2.Body = `{"log":"INFO: second line","stream":"stderr","time":"2029-03-30T08:31:21.545Z"}`

	entry3 := entry.New()
	entry3.Body = `{"log":"INFO: third line","stream":"stdout","time":"2029-03-30T08:31:22.545Z"}`

	testEntries := []*entry.Entry{entry1, entry2, entry3}

	err = op.ProcessBatch(ctx, testEntries)
	require.NoError(t, err)

	// Verify that ProcessBatch was called exactly once with all entries
	// This proves that the batch was not split into individual entries
	output.AssertCalled(t, "ProcessBatch", ctx, mock.MatchedBy(func(entries []*entry.Entry) bool {
		return len(entries) == 3
	}))
	output.AssertNumberOfCalls(t, "ProcessBatch", 1)
}

// TestDockerProcessBatchWithSkippedEntries verifies that when some entries are skipped
// by an if condition, the remaining entries are still processed as a batch.
func TestDockerProcessBatchWithSkippedEntries(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("CanProcess").Return(true)
	output.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

	cfg := NewConfigWithID("test_id")
	cfg.AddMetadataFromFilePath = false
	cfg.Format = "docker"
	cfg.IfExpr = `attributes["process"] == "true"`
	cfg.OutputIDs = []string{"test-output"}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	defer func() { require.NoError(t, op.Stop()) }()

	err = op.SetOutputs([]operator.Operator{output})
	require.NoError(t, err)

	ctx := t.Context()

	entry1 := entry.New()
	entry1.Body = `{"log":"INFO: first line","stream":"stdout","time":"2029-03-30T08:31:20.545Z"}`
	entry1.Attributes = map[string]any{"process": "true"}

	entry2 := entry.New()
	entry2.Body = `not a docker log - should be skipped`
	entry2.Attributes = map[string]any{"process": "false"}

	entry3 := entry.New()
	entry3.Body = `{"log":"INFO: third line","stream":"stdout","time":"2029-03-30T08:31:22.545Z"}`
	entry3.Attributes = map[string]any{"process": "true"}

	testEntries := []*entry.Entry{entry1, entry2, entry3}

	err = op.ProcessBatch(ctx, testEntries)
	require.NoError(t, err)

	// All entries (2 processed + 1 skipped) should be sent in a single batch
	output.AssertCalled(t, "ProcessBatch", ctx, mock.MatchedBy(func(entries []*entry.Entry) bool {
		return len(entries) == 3
	}))
	output.AssertNumberOfCalls(t, "ProcessBatch", 1)
}

// TestCRIProcessBatchDoesNotSplitBatches verifies that the container parser processes
// batches of CRI entries without splitting them.
func TestCRIProcessBatchDoesNotSplitBatches(t *testing.T) {
	cases := []struct {
		name           string
		format         string
		input          []*entry.Entry
		expectedOutput []*entry.Entry
	}{
		{
			name:   "crio_standalone_batch",
			format: "",
			input: []*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout F first crio line`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:38.505201169-10:00 stdout F second crio line`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			expectedOutput: []*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "F",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Body: "first crio line",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 37, 505201169, time.FixedZone("", -10*60*60)),
				},
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "F",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Body: "second crio line",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 38, 505201169, time.FixedZone("", -10*60*60)),
				},
			},
		},
		{
			name:   "containerd_standalone_batch",
			format: "",
			input: []*entry.Entry{
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F first containerd line`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:38.505201169Z stdout F second containerd line`,
					Attributes: map[string]any{
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			expectedOutput: []*entry.Entry{
				{
					Attributes: map[string]any{
						"log.iostream":    "stdout",
						"logtag":          "F",
						attrs.LogFilePath: "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
					Body: "first containerd line",
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
					Body: "second containerd line",
					Resource: map[string]any{
						"k8s.pod.name":                "kube-scheduler-kind-control-plane",
						"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
						"k8s.container.name":          "kube-scheduler44",
						"k8s.container.restart_count": "1",
						"k8s.namespace.name":          "some",
					},
					Timestamp: time.Date(2024, time.April, 13, 7, 59, 38, 505201169, time.UTC),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			cfg := NewConfigWithID("test_id")
			cfg.AddMetadataFromFilePath = true
			if tc.format != "" {
				cfg.Format = tc.format
			}
			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)
			defer func() { require.NoError(t, op.Stop()) }()
			r := op.(*Parser)

			fake := testutil.NewFakeOutput(t)
			r.OutputOperators = []operator.Operator{fake}

			err = r.ProcessBatch(ctx, tc.input)
			require.NoError(t, err)

			fake.ExpectEntries(t, tc.expectedOutput)

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

func TestMaxLogSizeRecombine(t *testing.T) {
	const (
		partialSize = 600 * 1024 // 600KB per partial entry
		oneMiB      = 1024 * 1024
	)

	filePath := "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log"
	largeContent := strings.Repeat("x", partialSize)

	makeCRIOEntry := func(content, tag string) *entry.Entry {
		return &entry.Entry{
			Body: fmt.Sprintf("2024-04-13T07:59:37.505201169-10:00 stdout %s %s", tag, content),
			Attributes: map[string]any{
				attrs.LogFilePath: filePath,
			},
		}
	}

	cases := []struct {
		name     string
		op       func() (operator.Operator, error)
		input    []*entry.Entry
		validate func(t *testing.T, fake *testutil.FakeOutput)
	}{
		{
			"default_1MiB_limit_flushes_oversized_logs",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				makeCRIOEntry(largeContent, "P"),
				makeCRIOEntry(largeContent, "P"),
				makeCRIOEntry("final", "F"),
			},
			func(t *testing.T, fake *testutil.FakeOutput) {
				// First entry: flushed due to size limit
				select {
				case e := <-fake.Received:
					body, _ := e.Body.(string)
					require.Greater(t, len(body), partialSize)
					require.Contains(t, e.Attributes, "log.iostream")
				case <-time.After(time.Second):
					require.FailNow(t, "Timed out waiting for first entry")
				}

				// Second entry: final content
				select {
				case e := <-fake.Received:
					body, _ := e.Body.(string)
					require.Equal(t, "final", body)
					require.Contains(t, e.Attributes, "log.iostream")
				case <-time.After(time.Second):
					require.FailNow(t, "Timed out waiting for second entry")
				}
			},
		},
		{
			"zero_allows_unlimited_batching",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.AddMetadataFromFilePath = true
				cfg.MaxLogSize = 0 // Unlimited
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			[]*entry.Entry{
				makeCRIOEntry(largeContent, "P"),
				makeCRIOEntry(largeContent, "P"),
				makeCRIOEntry("final", "F"),
			},
			func(t *testing.T, fake *testutil.FakeOutput) {
				// Single combined entry exceeding 1MiB
				select {
				case e := <-fake.Received:
					body, _ := e.Body.(string)
					require.Greater(t, len(body), oneMiB)
					require.Contains(t, body, "final")
					require.Contains(t, e.Attributes, "log.iostream")
				case <-time.After(time.Second):
					require.FailNow(t, "Timed out waiting for combined entry")
				}
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
			r.OutputOperators = []operator.Operator{fake}

			for _, e := range tc.input {
				require.NoError(t, r.Process(ctx, e))
			}

			tc.validate(t, fake)

			select {
			case e := <-fake.Received:
				require.FailNow(t, "Received unexpected entry: ", "%+v", e)
			default:
			}
		})
	}
}

func TestUnlimitedBatchSize(t *testing.T) {
	const (
		numPartialEntries = 1100
	)

	filePath := "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log"

	makeCRIOEntry := func(content, tag string) *entry.Entry {
		return &entry.Entry{
			Body: fmt.Sprintf("2024-04-13T07:59:37.505201169-10:00 stdout %s %s", tag, content),
			Attributes: map[string]any{
				attrs.LogFilePath: filePath,
			},
		}
	}

	ctx := t.Context()
	cfg := NewConfigWithID("test_id")
	cfg.AddMetadataFromFilePath = true
	cfg.MaxLogSize = 0
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	defer func() { require.NoError(t, op.Stop()) }()

	r := op.(*Parser)
	fake := testutil.NewFakeOutput(t)
	r.OutputOperators = []operator.Operator{fake}

	input := make([]*entry.Entry, 0, numPartialEntries+1)
	for i := range numPartialEntries {
		input = append(input, makeCRIOEntry(fmt.Sprintf("part%d", i), "P"))
	}
	input = append(input, makeCRIOEntry("final", "F"))

	for _, e := range input {
		require.NoError(t, r.Process(ctx, e))
	}

	select {
	case e := <-fake.Received:
		body, ok := e.Body.(string)
		require.True(t, ok)
		require.Contains(t, body, "part0", "Should contain first partial entry")
		require.Contains(t, body, "part1099", "Should contain last partial entry (1100th)")
		require.Contains(t, body, "final", "Should contain final entry")
		partCount := strings.Count(body, "part")
		require.Equal(t, numPartialEntries, partCount, "All %d partial entries should be in single combined log", numPartialEntries)
	case <-time.After(3 * time.Second):
		require.FailNow(t, "Timed out waiting for combined entry")
	}

	select {
	case e := <-fake.Received:
		require.FailNow(t, "Received unexpected second entry - batch was incorrectly split", "entry: %+v", e)
	default:
	}
}
