// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
