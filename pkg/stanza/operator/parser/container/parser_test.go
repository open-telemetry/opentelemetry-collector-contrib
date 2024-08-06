// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
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
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestConfigBuildFormatError(t *testing.T) {
	config := NewConfigWithID("test")
	config.Format = "invalid_runtime"
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `format` field")
}

func TestDockerParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parseDocker([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as docker container logs")
}

func TestCrioParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parseCRIO([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as cri-o container logs")
}

func TestContainerdParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parseContainerd([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as containerd logs")
}

func TestFormatDetectionFailure(t *testing.T) {
	parser := newTestParser(t)
	e := &entry.Entry{
		Body: `invalid container format`,
	}
	_, err := parser.detectFormat(e)
	require.Error(t, err)
	require.Contains(t, err.Error(), "entry cannot be parsed as container logs")
}

func TestInternalRecombineCfg(t *testing.T) {
	cfg := createRecombineConfig(Config{MaxLogSize: 102400})
	expected := recombine.NewConfigWithID(recombineInternalID)
	expected.IsLastEntry = "attributes.logtag == 'F'"
	expected.CombineField = entry.NewBodyField()
	expected.CombineWith = ""
	expected.SourceIdentifier = entry.NewAttributeField("log.file.path")
	expected.MaxLogSize = 102400
	require.Equal(t, cfg, expected)
}

func TestProcess(t *testing.T) {
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
					"time":         "2029-03-30T08:31:20.545192187Z",
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
					"time":         "2029-03-30T08:31:20.545192187Z",
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
					"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
				},
			},
			&entry.Entry{
				Attributes: map[string]any{
					"time":          "2029-03-30T08:31:20.545192187Z",
					"log.iostream":  "stdout",
					"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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

			err = op.Process(context.Background(), tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.input)
			// Stop the operator
			require.NoError(t, op.Stop())
		})
	}
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
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"time":          "2024-04-13T07:59:37.505201169-10:00",
						"log.iostream":  "stdout",
						"logtag":        "F",
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"time":          "2024-04-13T07:59:37.505201169Z",
						"log.iostream":  "stdout",
						"logtag":        "F",
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout F s awesome!`,
					Attributes: map[string]any{
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"time":          "2024-04-13T07:59:37.505201169-10:00",
						"log.iostream":  "stdout",
						"logtag":        "P",
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F s awesome!`,
					Attributes: map[string]any{
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"time":          "2024-04-13T07:59:37.505201169Z",
						"log.iostream":  "stdout",
						"logtag":        "P",
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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
			ctx := context.Background()
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
				require.FailNow(t, "Received unexpected entry: ", e)
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
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout F s awesome!`,
					Attributes: map[string]any{
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169-10:00 stdout F standalone crio2 line which is awesome!`,
					Attributes: map[string]any{
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"time":          "2024-04-13T07:59:37.505201169-10:00",
						"log.iostream":  "stdout",
						"logtag":        "P",
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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
						"time":          "2024-04-13T07:59:37.505201169-10:00",
						"log.iostream":  "stdout",
						"logtag":        "F",
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F s awesome!`,
					Attributes: map[string]any{
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
				{
					Body: `2024-04-13T07:59:37.505201169Z stdout F standalone containerd2 line which is awesome!`,
					Attributes: map[string]any{
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
					},
				},
			},
			[]*entry.Entry{
				{
					Attributes: map[string]any{
						"time":          "2024-04-13T07:59:37.505201169Z",
						"log.iostream":  "stdout",
						"logtag":        "P",
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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
						"time":          "2024-04-13T07:59:37.505201169Z",
						"log.iostream":  "stdout",
						"logtag":        "F",
						"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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
			ctx := context.Background()
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
				require.FailNow(t, "Received unexpected entry: ", e)
			default:
			}
		})
	}
}

func TestProcessWithTimeRemovalFlag(t *testing.T) {

	require.NoError(t, featuregate.GlobalRegistry().Set(removeOriginalTimeField.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(removeOriginalTimeField.ID(), false))
	})

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
					"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
				},
			},
			&entry.Entry{
				Attributes: map[string]any{
					"log.iostream":  "stdout",
					"log.file.path": "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler44/1.log",
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

			err = op.Process(context.Background(), tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.input)
			// Stop the operator
			require.NoError(t, op.Stop())
		})
	}
}
