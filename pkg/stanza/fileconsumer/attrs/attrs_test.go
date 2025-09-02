// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attrs

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
)

func TestResolver(t *testing.T) {
	t.Parallel()

	for i := 0; i < 64; i++ {
		// Create a 6 bit string where each bit represents the value of a config option
		bitString := fmt.Sprintf("%06b", i)

		// Create a resolver with a config that matches the bit pattern of i
		r := Resolver{
			IncludeFileName:           bitString[0] == '1',
			IncludeFilePath:           bitString[1] == '1',
			IncludeFileNameResolved:   bitString[2] == '1',
			IncludeFilePathResolved:   bitString[3] == '1',
			IncludeFileOwnerName:      bitString[4] == '1' && runtime.GOOS != "windows",
			IncludeFileOwnerGroupName: bitString[5] == '1' && runtime.GOOS != "windows",
			MetadataExtraction: MetadataExtraction{
				Regex: "^.*(\\/|\\\\)(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[a-f0-9\\-]+)(\\/|\\\\)(?P<container_name>[^\\._]+)(\\/|\\\\)(?P<restart_count>\\d+)\\.log(\\.\\d{8}-\\d{6})?$",
			},
		}

		t.Run(bitString, func(t *testing.T) {
			// Create a file
			tempDir := t.TempDir()
			temp := filetest.OpenTemp(t, tempDir)

			attributes, err := r.Resolve(temp)
			assert.NoError(t, err)

			var expectLen int
			if r.IncludeFileName {
				expectLen++
				assert.Equal(t, filepath.Base(temp.Name()), attributes[LogFileName])
			} else {
				assert.Empty(t, attributes[LogFileName])
			}
			if r.IncludeFilePath {
				expectLen++
				assert.Equal(t, temp.Name(), attributes[LogFilePath])
			} else {
				assert.Empty(t, attributes[LogFilePath])
			}

			// We don't have an independent way to resolve the path, so the only meaningful validate
			// is to ensure that the resolver returns nothing vs something based on the config.
			if r.IncludeFileNameResolved {
				expectLen++
				assert.NotNil(t, attributes[LogFileNameResolved])
				assert.IsType(t, "", attributes[LogFileNameResolved])
			} else {
				assert.Empty(t, attributes[LogFileNameResolved])
			}
			if r.IncludeFilePathResolved {
				expectLen++
				assert.NotNil(t, attributes[LogFilePathResolved])
				assert.IsType(t, "", attributes[LogFilePathResolved])
			} else {
				assert.Empty(t, attributes[LogFilePathResolved])
			}
			if r.IncludeFileOwnerName {
				expectLen++
				assert.NotNil(t, attributes[LogFileOwnerName])
				assert.IsType(t, "", attributes[LogFileOwnerName])
			} else {
				assert.Empty(t, attributes[LogFileOwnerName])
				assert.Empty(t, attributes[LogFileOwnerName])
			}
			if r.IncludeFileOwnerGroupName {
				expectLen++
				assert.NotNil(t, attributes[LogFileOwnerGroupName])
				assert.IsType(t, "", attributes[LogFileOwnerGroupName])
			} else {
				assert.Empty(t, attributes[LogFileOwnerGroupName])
				assert.Empty(t, attributes[LogFileOwnerGroupName])
			}
			assert.Len(t, attributes, expectLen)
		})
	}
}

func TestResolver_extractMetadata(t *testing.T) {
	type fields struct {
		MetadataExtraction MetadataExtraction
	}
	type args struct {
		path       string
		attributes map[string]any
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]any
		wantErr assert.ErrorAssertionFunc
	}{
		{
			args: args{
				path:       "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler/1.log",
				attributes: map[string]any{},
			},
			fields: fields{
				MetadataExtraction: MetadataExtraction{
					Regex: "^.*(\\/|\\\\)(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[a-f0-9\\-]+)(\\/|\\\\)(?P<container_name>[^\\._]+)(\\/|\\\\)(?P<restart_count>\\d+)\\.log(\\.\\d{8}-\\d{6})?$",
				},
			},
			want: map[string]any{
				"namespace":      "some",
				"pod_name":       "kube-scheduler-kind-control-plane",
				"uid":            "49cc7c1fd3702c40b2686ea7486091d3",
				"container_name": "kube-scheduler",
				"restart_count":  "1",
			},
			wantErr: assert.NoError,
		},
		{
			args: args{
				path:       "/var/log/pods/some_kube-scheduler-kind-control-plane_49cc7c1fd3702c40b2686ea7486091d3/kube-scheduler/1.log",
				attributes: map[string]any{},
			},
			fields: fields{
				MetadataExtraction: MetadataExtraction{
					Regex: "^.*(\\/|\\\\)(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[a-f0-9\\-]+)(\\/|\\\\)(?P<container_name>[^\\._]+)(\\/|\\\\)(?P<restart_count>\\d+)\\.log(\\.\\d{8}-\\d{6})?$",
					Mapping: map[string]string{
						"container_name": "k8s.container.name",
						"namespace":      "k8s.namespace.name",
						"pod_name":       "k8s.pod.name",
						"restart_count":  "k8s.container.restart_count",
						"uid":            "k8s.pod.uid",
					},
				},
			},
			want: map[string]any{
				"k8s.namespace.name":          "some",
				"k8s.pod.name":                "kube-scheduler-kind-control-plane",
				"k8s.pod.uid":                 "49cc7c1fd3702c40b2686ea7486091d3",
				"k8s.container.name":          "kube-scheduler",
				"k8s.container.restart_count": "1",
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Resolver{
				MetadataExtraction: tt.fields.MetadataExtraction,
			}
			got, err := r.extractMetadata(tt.args.path, tt.args.attributes)
			if !tt.wantErr(t, err, fmt.Sprintf("extractMetadata(%v, %v)", tt.args.path, tt.args.attributes)) {
				return
			}
			assert.Equalf(t, tt.want, got, "extractMetadata(%v, %v)", tt.args.path, tt.args.attributes)
		})
	}
}

func BenchmarkResolveWithMetadataExtraction(b *testing.B) {

	r := Resolver{
		// /var/folders/62/9h0qt0r51h5gzvxshbzl6ksh0000gq/T/BenchmarkResolveWithMetadataExtraction406651325/001/1725504552
		MetadataExtraction: MetadataExtraction{
			Regex: `^(?P<file>.*)$`,
		},
	}

	// Create a file
	tempDir := b.TempDir()
	temp := filetest.OpenTemp(b, tempDir)

	for i := 0; i < b.N; i++ {
		_, err := r.Resolve(temp)
		assert.NoError(b, err)
	}
}
