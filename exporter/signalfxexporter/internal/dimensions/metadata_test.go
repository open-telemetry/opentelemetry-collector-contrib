// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

func TestGetDimensionUpdateFromMetadata(t *testing.T) {
	translator, _ := translation.NewMetricTranslator([]translation.Rule{
		{
			Action:  translation.ActionRenameDimensionKeys,
			Mapping: map[string]string{"name": "translated_name"},
		},
	}, 1)
	type args struct {
		metadata         metadata.MetadataUpdate
		metricTranslator *translation.MetricTranslator
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
				metricTranslator: nil,
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
				metricTranslator: nil,
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
				metricTranslator: nil,
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
			"Test dimensions translation",
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
							"prope_rty2": "value2",
							"ta/g2":      "",
						},
						MetadataToUpdate: map[string]string{
							"prope.rty3": "value33",
							"prope.rty4": "",
						},
					},
				},
				metricTranslator: translator,
			},
			&DimensionUpdate{
				Name:  "translated_name",
				Value: "val",
				Properties: getMapToPointers(map[string]string{
					"prope/rty1": "value1",
					"prope_rty2": "",
					"prope.rty3": "value33",
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
				metricTranslator: nil,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			converter, err := translation.NewMetricsConverter(
				zap.NewNop(),
				tt.args.metricTranslator,
				nil,
				nil,
				"-_.",
			)
			require.NoError(t, err)
			assert.Equal(t, tt.want, getDimensionUpdateFromMetadata(tt.args.metadata, *converter))
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
