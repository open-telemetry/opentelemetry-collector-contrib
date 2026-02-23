// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

func TestGetDimensionUpdateFromMetadata(t *testing.T) {
	type args struct {
		defaults map[string]string
		metadata metadata.MetadataUpdate
	}
	tests := []struct {
		name string
		args args
		want *DimensionUpdate
	}{
		{
			"Test tags update",
			args{
				metadata: metadata.MetadataUpdate{
					ResourceIDKey: "name",
					ResourceID:    "val",
					MetadataDelta: metadata.MetadataDelta{
						MetadataToAdd: map[string]string{
							"tag1": "",
						},
						MetadataToRemove: map[string]string{
							"tag2": "",
						},
						MetadataToUpdate: map[string]string{},
					},
				},
			},
			&DimensionUpdate{
				Name:       "name",
				Value:      "val",
				Properties: map[string]*string{},
				Tags: map[string]bool{
					"tag1": true,
					"tag2": false,
				},
			},
		},
		{
			"Test properties update",
			args{
				metadata: metadata.MetadataUpdate{
					ResourceIDKey: "name",
					ResourceID:    "val",
					MetadataDelta: metadata.MetadataDelta{
						MetadataToAdd: map[string]string{
							"property1": "value1",
						},
						MetadataToRemove: map[string]string{
							"property2": "value2",
						},
						MetadataToUpdate: map[string]string{
							"property3": "value33",
							"property4": "",
						},
					},
				},
			},
			&DimensionUpdate{
				Name:  "name",
				Value: "val",
				Properties: getMapToPointers(map[string]string{
					"property1": "value1",
					"property2": "",
					"property3": "value33",
					"property4": "",
				}),
				Tags: map[string]bool{},
			},
		},
		{
			"Test with special characters",
			args{
				metadata: metadata.MetadataUpdate{
					ResourceIDKey: "name",
					ResourceID:    "val",
					MetadataDelta: metadata.MetadataDelta{
						MetadataToAdd: map[string]string{
							"prope/rty1": "value1",
							"ta.g1":      "",
						},
						MetadataToRemove: map[string]string{
							"prope.rty2": "value2",
							"ta/g2":      "",
						},
						MetadataToUpdate: map[string]string{
							"prope_rty3": "value33",
							"prope.rty4": "",
						},
					},
				},
			},
			&DimensionUpdate{
				Name:  "name",
				Value: "val",
				Properties: getMapToPointers(map[string]string{
					"prope/rty1": "value1",
					"prope.rty2": "",
					"prope_rty3": "value33",
					"prope.rty4": "",
				}),
				Tags: map[string]bool{
					"ta.g1": true,
					"ta/g2": false,
				},
			},
		},
		{
			"Test with k8s service properties",
			args{
				metadata: metadata.MetadataUpdate{
					ResourceIDKey: "name",
					ResourceID:    "val",
					MetadataDelta: metadata.MetadataDelta{
						MetadataToAdd: map[string]string{
							"k8s.service.ta.g5": "",
						},
						MetadataToRemove: map[string]string{
							"k8s.service.ta.g6": "",
						},
					},
				},
			},
			&DimensionUpdate{
				Name:       "name",
				Value:      "val",
				Properties: map[string]*string{},
				Tags: map[string]bool{
					"kubernetes_service_ta.g5": true,
					"kubernetes_service_ta.g6": false,
				},
			},
		},
		{
			"Test with defaults",
			args{
				defaults: map[string]string{
					"foo": "bar",
					"bar": "baz",
				},
				metadata: metadata.MetadataUpdate{
					ResourceIDKey: "name",
					ResourceID:    "val",
					MetadataDelta: metadata.MetadataDelta{
						MetadataToAdd: map[string]string{
							"bar": "foobar",
						},
					},
				},
			},
			&DimensionUpdate{
				Name:  "name",
				Value: "val",
				Properties: getMapToPointers(map[string]string{
					"foo": "bar",
					"bar": "foobar",
				}),
				Tags: map[string]bool{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, getDimensionUpdateFromMetadata(tt.args.defaults, tt.args.metadata, "-_."))
		})
	}
}

func getMapToPointers(m map[string]string) map[string]*string {
	out := map[string]*string{}

	for k, v := range m {
		if v == "" {
			out[k] = nil
		} else {
			propVal := v
			out[k] = &propVal
		}
	}

	return out
}

func TestFilterKeyChars(t *testing.T) {
	tests := []struct {
		name                    string
		nonAlphanumericDimChars string
		dim                     string
		want                    string
	}{
		{
			name:                    "periods_replaced_with_underscores",
			nonAlphanumericDimChars: "_-",
			dim:                     "d.i.m",
			want:                    "d_i_m",
		},
		{
			name:                    "periods_allowed_when_specified",
			nonAlphanumericDimChars: "_-.",
			dim:                     "d.i.m",
			want:                    "d.i.m",
		},
		{
			name:                    "multiple_special_chars_replaced",
			nonAlphanumericDimChars: "_",
			dim:                     "my-dim.name",
			want:                    "my_dim_name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterKeyChars(tt.dim, tt.nonAlphanumericDimChars)
			assert.Equal(t, tt.want, got)
		})
	}
}
