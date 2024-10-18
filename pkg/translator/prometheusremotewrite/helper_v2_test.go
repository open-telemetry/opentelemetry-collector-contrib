// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Test_createLabelSet checks resultant label names are sanitized and label in extra overrides label in labels if
// collision happens. It does not check whether labels are not sorted
func Test_createLabelSetV2(t *testing.T) {
	tests := []struct {
		name           string
		resource       pcommon.Resource
		orig           pcommon.Map
		externalLabels map[string]string
		extras         []string
		want           labels.Labels
	}{
		{
			"labels_clean",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31, label32, value32),
		},
		{
			"labels_with_resource",
			func() pcommon.Resource {
				res := pcommon.NewResource()
				res.Attributes().PutStr("service.name", "prometheus")
				res.Attributes().PutStr("service.instance.id", "127.0.0.1:8080")
				return res
			}(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31, label32, value32, "job", "prometheus", "instance", "127.0.0.1:8080"),
		},
		{
			"labels_with_nonstring_resource",
			func() pcommon.Resource {
				res := pcommon.NewResource()
				res.Attributes().PutInt("service.name", 12345)
				res.Attributes().PutBool("service.instance.id", true)
				return res
			}(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31, label32, value32, "job", "12345", "instance", "true"),
		},
		{
			"labels_duplicate_in_extras",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{label11, value31},
			getPromLabelsV2(label11, value31, label12, value12),
		},
		{
			"labels_dirty",
			pcommon.NewResource(),
			lbs1Dirty,
			map[string]string{},
			[]string{label31 + dirty1, value31, label32, value32},
			getPromLabelsV2(label11+"_", value11, "key_"+label12, value12, label31+"_", value31, label32, value32),
		},
		{
			"no_original_case",
			pcommon.NewResource(),
			pcommon.NewMap(),
			nil,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label31, value31, label32, value32),
		},
		{
			"empty_extra_case",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{"", ""},
			getPromLabelsV2(label11, value11, label12, value12, "", ""),
		},
		{
			"single_left_over_case",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31),
		},
		{
			"valid_external_labels",
			pcommon.NewResource(),
			lbs1,
			exlbs1,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label41, value41, label31, value31, label32, value32),
		},
		{
			"overwritten_external_labels",
			pcommon.NewResource(),
			lbs1,
			exlbs2,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, label31, value31, label32, value32),
		},
		{
			"colliding attributes",
			pcommon.NewResource(),
			lbsColliding,
			nil,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(collidingSanitized, value11+";"+value12, label31, value31, label32, value32),
		},
		{
			"sanitize_labels_starts_with_underscore",
			pcommon.NewResource(),
			lbs3,
			exlbs1,
			[]string{label31, value31, label32, value32},
			getPromLabelsV2(label11, value11, label12, value12, "key"+label51, value51, label41, value41, label31, value31, label32, value32),
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := createAttributesV2(tt.resource, tt.orig, tt.externalLabels, nil, true, tt.extras...)
			assert.ElementsMatch(t, tt.want, res)
		})
	}
}
