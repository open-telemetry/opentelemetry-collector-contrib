// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package influxdbexporter

import (
	"testing"

	"github.com/influxdata/influxdb-observability/common"
	"github.com/stretchr/testify/assert"
)

func Test_influxHTTPWriterBatch_optimizeTags(t *testing.T) {
	batch := &influxHTTPWriterBatch{
		influxHTTPWriter: &influxHTTPWriter{
			logger: common.NoopLogger{},
		},
	}

	for _, testCase := range []struct {
		name         string
		m            map[string]string
		expectedTags []tag
	}{
		{
			name:         "empty map",
			m:            map[string]string{},
			expectedTags: []tag{},
		},
		{
			name: "one tag",
			m: map[string]string{
				"k": "v",
			},
			expectedTags: []tag{
				{"k", "v"},
			},
		},
		{
			name: "empty tag key",
			m: map[string]string{
				"": "v",
			},
			expectedTags: []tag{},
		},
		{
			name: "empty tag value",
			m: map[string]string{
				"k": "",
			},
			expectedTags: []tag{},
		},
		{
			name: "seventeen tags",
			m: map[string]string{
				"k00": "v00", "k01": "v01", "k02": "v02", "k03": "v03", "k04": "v04", "k05": "v05", "k06": "v06", "k07": "v07", "k08": "v08", "k09": "v09", "k10": "v10", "k11": "v11", "k12": "v12", "k13": "v13", "k14": "v14", "k15": "v15", "k16": "v16",
			},
			expectedTags: []tag{
				{"k00", "v00"}, {"k01", "v01"}, {"k02", "v02"}, {"k03", "v03"}, {"k04", "v04"}, {"k05", "v05"}, {"k06", "v06"}, {"k07", "v07"}, {"k08", "v08"}, {"k09", "v09"}, {"k10", "v10"}, {"k11", "v11"}, {"k12", "v12"}, {"k13", "v13"}, {"k14", "v14"}, {"k15", "v15"}, {"k16", "v16"},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			gotTags := batch.optimizeTags(testCase.m)
			assert.Equal(t, testCase.expectedTags, gotTags)
		})
	}
}
