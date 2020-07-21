// Copyright 2019, OpenTelemetry Authors
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

package translation

import (
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricTranslator(t *testing.T) {
	tests := []struct {
		name              string
		trs               []Rule
		wantDimensionsMap map[string]string
		wantError         string
	}{
		{
			name: "invalid_rule",
			trs: []Rule{
				{
					Action: "invalid_rule",
				},
			},
			wantDimensionsMap: nil,
			wantError:         "unknown \"action\" value: \"invalid_rule\"",
		},
		{
			name: "rename_dimension_keys_valid",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"k8s.cluster.name": "kubernetes_cluster",
					},
				},
			},
			wantDimensionsMap: map[string]string{
				"k8s.cluster.name": "kubernetes_cluster",
			},
			wantError: "",
		},
		{
			name: "rename_dimension_keys_no_mapping",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "field \"mapping\" is required for \"rename_dimension_keys\" translation rule",
		},
		{
			name: "rename_dimension_keys_many_actions_invalid",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"dimension1": "dimension2",
						"dimension3": "dimension4",
					},
				},
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"dimension4": "dimension5",
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "only one \"rename_dimension_keys\" translation rule can be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := NewMetricTranslator(tt.trs)
			if tt.wantError == "" {
				require.NoError(t, err)
				require.NotNil(t, mt)
				assert.Equal(t, tt.trs, mt.rules)
				assert.Equal(t, tt.wantDimensionsMap, mt.dimensionsMap)
			} else {
				require.Error(t, err)
				assert.Equal(t, err.Error(), tt.wantError)
				require.Nil(t, mt)
			}
		})
	}
}

func TestTranslateDataPoints(t *testing.T) {
	msec := time.Now().Unix() * 1e3
	gaugeType := sfxpb.MetricType_GAUGE

	tests := []struct {
		name string
		trs  []Rule
		dps  []*sfxpb.DataPoint
		want []*sfxpb.DataPoint
	}{
		{
			name: "rename_dimension_keys",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"old_dimension": "new_dimension",
						"old.dimension": "new.dimension",
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "single",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "old_dimension",
							Value: "value1",
						},
						{
							Key:   "old.dimension",
							Value: "value2",
						},
						{
							Key:   "dimension",
							Value: "value3",
						},
					},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "single",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "new_dimension",
							Value: "value1",
						},
						{
							Key:   "new.dimension",
							Value: "value2",
						},
						{
							Key:   "dimension",
							Value: "value3",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := NewMetricTranslator(tt.trs)
			require.NoError(t, err)
			assert.NotEqualValues(t, tt.want, tt.dps)
			mt.TranslateDataPoints(tt.dps)

			for i, dp := range tt.dps {
				if dp.GetValue().DoubleValue != nil {
					assert.InDelta(t, *tt.want[i].GetValue().DoubleValue, *dp.GetValue().DoubleValue, 0.00000001)
					*dp.GetValue().DoubleValue = *tt.want[i].GetValue().DoubleValue
				}
			}

			assert.EqualValues(t, tt.want, tt.dps)
		})
	}
}

func TestTestTranslateDimension(t *testing.T) {
	mt, err := NewMetricTranslator([]Rule{
		{
			Action: ActionRenameDimensionKeys,
			Mapping: map[string]string{
				"old_dimension": "new_dimension",
				"old.dimension": "new.dimension",
			},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "new_dimension", mt.TranslateDimension("old_dimension"))
	assert.Equal(t, "new.dimension", mt.TranslateDimension("old.dimension"))
	assert.Equal(t, "another_dimension", mt.TranslateDimension("another_dimension"))

	// Test no rename_dimension_keys translation rule
	mt, err = NewMetricTranslator([]Rule{})
	require.NoError(t, err)
	assert.Equal(t, "old_dimension", mt.TranslateDimension("old_dimension"))
}

func generateIntPtr(i int) *int64 {
	var iPtr int64 = int64(i)
	return &iPtr
}
