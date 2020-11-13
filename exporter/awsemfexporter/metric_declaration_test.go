// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLabelMatcherInit(t *testing.T) {
	lm := &LabelMatcher{
		LabelNames: []string{"label1", "label2"},
	}
	err := lm.Init()
	assert.Nil(t, err)
	assert.Equal(t, ";", lm.Separator)
	assert.Equal(t, ".+", lm.Regex)
	assert.NotNil(t, lm.compiledRegex)

	lm.Separator = ""
	err = lm.Init()
	assert.Nil(t, err)
	assert.Equal(t, ";", lm.Separator)

	lm.Separator = ","
	err = lm.Init()
	assert.Nil(t, err)
	assert.Equal(t, ",", lm.Separator)

	lm.Regex = "a*"
	err = lm.Init()
	assert.Nil(t, err)
	assert.Equal(t, "a*", lm.Regex)
	assert.NotNil(t, lm.compiledRegex)

	// Test error
	lm.LabelNames = []string{}
	err = lm.Init()
	assert.NotNil(t, err)
	assert.EqualError(t, err, "label matcher must have at least one label name specified")
}

func TestGetConcatenatedLabels(t *testing.T) {
	labels := map[string]string{
		"label1": "a",
		"label2": "b",
		"label3": "c",
	}
	testCases := []struct {
		testName   string
		labelNames []string
		separator  string
		expected   string
	}{
		{
			"Single label w/ default separator",
			[]string{"label1"},
			"",
			"a",
		},
		{
			"Multiple labels w/ default separator",
			[]string{"label1", "label3", "label2"},
			"",
			"a;c;b",
		},
		{
			"Single label w/ custom separator",
			[]string{"label1"},
			",",
			"a",
		},
		{
			"Multiple labels w/ custom separator",
			[]string{"label1", "label3", "label2"},
			",",
			"a,c,b",
		},
		{
			"Multiple labels w/ missing label",
			[]string{"label1", "label4", "label2"},
			"",
			"a;;b",
		},
	}

	for _, tc := range testCases {
		lm := &LabelMatcher{
			LabelNames: tc.labelNames,
			Separator:  tc.separator,
		}
		lm.Init()
		t.Run(tc.testName, func(t *testing.T) {
			concatenatedLabels := lm.getConcatenatedLabels(labels)
			assert.Equal(t, tc.expected, concatenatedLabels)
		})
	}
}

func TestLabelMatcherMatches(t *testing.T) {
	testCases := []struct {
		testName     string
		labels       map[string]string
		labelMatcher *LabelMatcher
		expected     bool
	}{
		{
			"Single label",
			map[string]string{
				"label1": "foo",
			},
			&LabelMatcher{
				LabelNames: []string{"label1"},
				Regex:      "^fo+$",
			},
			true,
		},
		{
			"Single label, no match",
			map[string]string{
				"label1": "foo",
			},
			&LabelMatcher{
				LabelNames: []string{"label1"},
				Regex:      "^f+$",
			},
			false,
		},
		{
			"Single label w/ missing label name",
			map[string]string{
				"label1": "foo",
			},
			&LabelMatcher{
				LabelNames: []string{"label2"},
			},
			false,
		},
		{
			"Multiple labels",
			map[string]string{
				"label1": "foo",
				"label2": "bar",
				"label3": "car",
			},
			&LabelMatcher{
				LabelNames: []string{"label1", "label3", "label2"},
				Regex:      "fo+;car;b.*$",
			},
			true,
		},
		{
			"Multiple labels w/ custom separator",
			map[string]string{
				"label1": "foo",
				"label2": "bar",
				"label3": "car",
			},
			&LabelMatcher{
				LabelNames: []string{"label1", "label3", "label2"},
				Separator:  ",",
				Regex:      "fo+,car,b.*$",
			},
			true,
		},
		{
			"Multiple labels, no match",
			map[string]string{
				"label1": "foo",
				"label2": "bar",
				"label3": "car",
			},
			&LabelMatcher{
				LabelNames: []string{"label1", "label3", "label2"},
				Separator:  ",",
				Regex:      "fo+;car;b.*$",
			},
			false,
		},
		{
			"Multiple labels w/ missing label name",
			map[string]string{
				"label1": "foo",
				"label2": "bar",
				"label3": "car",
			},
			&LabelMatcher{
				LabelNames: []string{"label1", "label4", "label2"},
				Regex:      "fo+;;b.*$",
			},
			true,
		},
	}

	for _, tc := range testCases {
		tc.labelMatcher.Init()
		t.Run(tc.testName, func(t *testing.T) {
			matches := tc.labelMatcher.Matches(tc.labels)
			assert.Equal(t, tc.expected, matches)
		})
	}
}

func TestMetricDeclarationInit(t *testing.T) {
	logger := zap.NewNop()
	t.Run("no dimensions", func(t *testing.T) {
		m := &MetricDeclaration{
			MetricNameSelectors: []string{"a", "b", "aa"},
		}
		err := m.Init(logger)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(m.metricRegexList))
	})

	t.Run("with dimensions", func(t *testing.T) {
		m := &MetricDeclaration{
			Dimensions: [][]string{
				{"foo"},
				{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			},
			MetricNameSelectors: []string{"a.*", "b$", "aa+"},
		}
		err := m.Init(logger)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(m.metricRegexList))
		assert.Equal(t, 2, len(m.Dimensions))
	})

	// Test removal of dimension sets with more than 10 elements
	t.Run("dimension set with more than 10 elements", func(t *testing.T) {
		m := &MetricDeclaration{
			Dimensions: [][]string{
				{"foo"},
				{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			},
			MetricNameSelectors: []string{"a.*", "b$", "aa+"},
		}
		obs, logs := observer.New(zap.WarnLevel)
		obsLogger := zap.New(obs)
		err := m.Init(obsLogger)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(m.metricRegexList))
		assert.Equal(t, 1, len(m.Dimensions))
		// Check logged warning message
		expectedLogs := []observer.LoggedEntry{{
			Entry:   zapcore.Entry{Level: zap.WarnLevel, Message: "Dropped dimension set: > 10 dimensions specified."},
			Context: []zapcore.Field{zap.String("dimensions", "a,b,c,d,e,f,g,h,i,j,k")},
		}}
		assert.Equal(t, 1, logs.Len())
		assert.Equal(t, expectedLogs, logs.AllUntimed())
	})

	// Test removal of duplicate dimensions within a dimension set, and removal of
	// duplicate dimension sets
	t.Run("remove duplicate dimensions and dimension sets", func(t *testing.T) {
		m := &MetricDeclaration{
			Dimensions: [][]string{
				{"a", "c", "b", "c"},
				{"c", "b", "a"},
			},
			MetricNameSelectors: []string{"a.*", "b$", "aa+"},
		}
		obs, logs := observer.New(zap.DebugLevel)
		obsLogger := zap.New(obs)
		err := m.Init(obsLogger)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(m.Dimensions))
		assert.Equal(t, []string{"a", "b", "c"}, m.Dimensions[0])
		// Check logged warning message
		expectedLogs := []observer.LoggedEntry{
			{
				Entry:   zapcore.Entry{Level: zap.DebugLevel, Message: "Removed duplicates from dimension set."},
				Context: []zapcore.Field{zap.String("dimensions", "a,c,b,c")},
			},
			{
				Entry:   zapcore.Entry{Level: zap.DebugLevel, Message: "Dropped dimension set: duplicated dimension set."},
				Context: []zapcore.Field{zap.String("dimensions", "c,b,a")},
			},
		}
		assert.Equal(t, 2, logs.Len())
		assert.Equal(t, expectedLogs, logs.AllUntimed())
	})

	// Test invalid metric declaration
	t.Run("invalid metric declaration", func(t *testing.T) {
		m := &MetricDeclaration{}
		err := m.Init(logger)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "invalid metric declaration: no metric name selectors defined")
	})

	// Test initialization of label matchers
	t.Run("initialization of label matchers", func(t *testing.T) {
		m := &MetricDeclaration{
			MetricNameSelectors: []string{"foo"},
			LabelMatchers: []*LabelMatcher{
				{
					LabelNames: []string{"label1", "label2"},
				},
				{
					LabelNames: []string{"label1", "label3"},
					Separator:  ",",
					Regex:      "a*",
				},
			},
		}
		err := m.Init(logger)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(m.LabelMatchers))
		assert.Equal(t, ";", m.LabelMatchers[0].Separator)
		assert.Equal(t, ".+", m.LabelMatchers[0].Regex)
		assert.NotNil(t, m.LabelMatchers[0].compiledRegex)
		assert.Equal(t, ",", m.LabelMatchers[1].Separator)
		assert.Equal(t, "a*", m.LabelMatchers[1].Regex)
		assert.NotNil(t, m.LabelMatchers[1].compiledRegex)
	})

	// Test error from label matcher initialization
	t.Run("label matcher init error", func(t *testing.T) {
		m := &MetricDeclaration{
			MetricNameSelectors: []string{"foo"},
			LabelMatchers: []*LabelMatcher{
				{
					LabelNames: []string{"label1", "label2"},
				},
				{
					LabelNames: []string{},
				},
			},
		}
		err := m.Init(logger)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "label matcher must have at least one label name specified")
	})
}

func TestMetricDeclarationMatches(t *testing.T) {
	m := &MetricDeclaration{
		MetricNameSelectors: []string{"^a+$", "^b.*$", "^ac+a$"},
	}
	logger := zap.NewNop()
	err := m.Init(logger)
	assert.Nil(t, err)

	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("a")
	assert.True(t, m.Matches(&metric, nil))

	metric.SetName("aa")
	assert.True(t, m.Matches(&metric, nil))

	metric.SetName("aaaa")
	assert.True(t, m.Matches(&metric, nil))

	metric.SetName("aaab")
	assert.False(t, m.Matches(&metric, nil))

	metric.SetName("b")
	assert.True(t, m.Matches(&metric, nil))

	metric.SetName("ba")
	assert.True(t, m.Matches(&metric, nil))

	metric.SetName("c")
	assert.False(t, m.Matches(&metric, nil))

	metric.SetName("aca")
	assert.True(t, m.Matches(&metric, nil))

	metric.SetName("accca")
	assert.True(t, m.Matches(&metric, nil))
}

func TestMetricDeclarationMatchesWithLabelMatchers(t *testing.T) {
	labels := map[string]string{
		"label1": "foo",
		"label2": "bar",
		"label3": "car",
	}
	testCases := []struct {
		testName      string
		labelMatchers []*LabelMatcher
		expected      bool
	}{
		{
			"Single label",
			[]*LabelMatcher{
				{
					LabelNames: []string{"label1"},
					Regex:      "^fo+$",
				},
			},
			true,
		},
		{
			"Multiple matchers w/ single label",
			[]*LabelMatcher{
				{
					LabelNames: []string{"label1"},
					Regex:      "food",
				},
				{
					LabelNames: []string{"label3"},
					Regex:      "^c.*$",
				},
			},
			true,
		},
		{
			"Multiple matchers w/ single label, no match",
			[]*LabelMatcher{
				{
					LabelNames: []string{"label1"},
					Regex:      "food",
				},
				{
					LabelNames: []string{"label3"},
					Regex:      "cat",
				},
			},
			false,
		},
		{
			"Multiple labels",
			[]*LabelMatcher{
				{
					LabelNames: []string{"label1", "label3", "label2"},
					Regex:      "fo+;car;b.*$",
				},
			},
			true,
		},
		{
			"Multiple labels w/ custom separator",
			[]*LabelMatcher{
				{
					LabelNames: []string{"label1", "label3", "label2"},
					Separator:  ",",
					Regex:      "fo+,car,b.*$",
				},
			},
			true,
		},
		{
			"Multiple matchers w/ multiple labels",
			[]*LabelMatcher{
				{
					LabelNames: []string{"label1", "label3", "label2"},
					Separator:  ",",
					Regex:      "fo+,car,b.*$",
				},
				{
					LabelNames: []string{"label3"},
					Regex:      "fat",
				},
			},
			true,
		},
		{
			"Multiple matchers w/ multiple labels, no match",
			[]*LabelMatcher{
				{
					LabelNames: []string{"label1", "label3", "label2"},
					Separator:  ",",
					Regex:      "fo+,cat,b.*$",
				},
				{
					LabelNames: []string{"label3"},
					Regex:      "fat",
				},
			},
			false,
		},
	}
	logger := zap.NewNop()
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("a")

	for _, tc := range testCases {
		m := MetricDeclaration{
			MetricNameSelectors: []string{"^a+$"},
			LabelMatchers:       tc.labelMatchers,
		}
		t.Run(tc.testName, func(t *testing.T) {
			err := m.Init(logger)
			assert.Nil(t, err)
			matches := m.Matches(&metric, labels)
			assert.Equal(t, tc.expected, matches)
		})
	}
}

func TestExtractDimensions(t *testing.T) {
	testCases := []struct {
		testName            string
		dimensions          [][]string
		labels              map[string]string
		extractedDimensions [][]string
	}{
		{
			"matches single dimension set exactly",
			[][]string{{"a", "b"}},
			map[string]string{
				"a": "foo",
				"b": "bar",
			},
			[][]string{{"a", "b"}},
		},
		{
			"matches subset of single dimension set",
			[][]string{{"a"}},
			map[string]string{
				"a": "foo",
				"b": "bar",
			},
			[][]string{{"a"}},
		},
		{
			"does not match single dimension set",
			[][]string{{"a", "b"}},
			map[string]string{
				"b": "bar",
			},
			nil,
		},
		{
			"matches multiple dimension sets",
			[][]string{{"a", "b"}, {"a"}},
			map[string]string{
				"a": "foo",
				"b": "bar",
			},
			[][]string{{"a", "b"}, {"a"}},
		},
		{
			"matches one of multiple dimension sets",
			[][]string{{"a", "b"}, {"a"}},
			map[string]string{
				"a": "foo",
			},
			[][]string{{"a"}},
		},
		{
			"no dimensions",
			[][]string{},
			map[string]string{
				"a": "foo",
			},
			nil,
		},
		{
			"empty dimension set",
			[][]string{{}},
			map[string]string{
				"a": "foo",
			},
			nil,
		},
	}
	logger := zap.NewNop()

	for _, tc := range testCases {
		m := MetricDeclaration{
			Dimensions:          tc.dimensions,
			MetricNameSelectors: []string{"foo"},
		}
		t.Run(tc.testName, func(t *testing.T) {
			err := m.Init(logger)
			assert.Nil(t, err)
			dimensions := m.ExtractDimensions(tc.labels)
			assert.Equal(t, tc.extractedDimensions, dimensions)
		})
	}
}

func TestProcessMetricDeclarations(t *testing.T) {
	metricDeclarations := []*MetricDeclaration{
		{
			Dimensions:          [][]string{{"dim1", "dim2"}},
			MetricNameSelectors: []string{"a", "b", "c"},
		},
		{
			Dimensions:          [][]string{{"dim1"}},
			MetricNameSelectors: []string{"aa", "b"},
		},
		{
			Dimensions:          [][]string{{"dim2", "dim1"}, {"dim1"}},
			MetricNameSelectors: []string{"a"},
		},
	}
	logger := zap.NewNop()
	for _, m := range metricDeclarations {
		err := m.Init(logger)
		assert.Nil(t, err)
	}
	testCases := []struct {
		testName     string
		metricName   string
		labels       map[string]string
		rollUpDims   [][]string
		expectedDims [][]string
	}{
		{
			"Matching single declaration",
			"c",
			map[string]string{
				"dim1": "foo",
				"dim2": "bar",
			},
			nil,
			[][]string{
				{"dim1", "dim2"},
			},
		},
		{
			"Match single dimension set",
			"a",
			map[string]string{
				"dim1": "foo",
			},
			nil,
			[][]string{
				{"dim1"},
			},
		},
		{
			"Match single dimension set w/ rolled-up dims",
			"a",
			map[string]string{
				"dim1": "foo",
				"dim3": "car",
			},
			[][]string{{"dim1"}, {"dim3"}},
			[][]string{
				{"dim1"},
				{"dim3"},
			},
		},
		{
			"Matching multiple declarations",
			"b",
			map[string]string{
				"dim1": "foo",
				"dim2": "bar",
			},
			nil,
			[][]string{
				{"dim1", "dim2"},
				{"dim1"},
			},
		},
		{
			"Matching multiple declarations w/ duplicate",
			"a",
			map[string]string{
				"dim1": "foo",
				"dim2": "bar",
			},
			nil,
			[][]string{
				{"dim1", "dim2"},
				{"dim1"},
			},
		},
		{
			"Matching multiple declarations w/ duplicate + rolled-up dims",
			"a",
			map[string]string{
				"dim1": "foo",
				"dim2": "bar",
				"dim3": "car",
			},
			[][]string{{"dim2", "dim1"}, {"dim3"}},
			[][]string{
				{"dim1", "dim2"},
				{"dim1"},
				{"dim3"},
			},
		},
		{
			"No matching dimension set",
			"a",
			map[string]string{
				"dim2": "bar",
			},
			nil,
			nil,
		},
		{
			"No matching dimension set w/ rolled-up dims",
			"a",
			map[string]string{
				"dim2": "bar",
			},
			[][]string{{"dim2"}},
			[][]string{{"dim2"}},
		},
		{
			"No matching metric name",
			"c",
			map[string]string{
				"dim1": "foo",
			},
			nil,
			nil,
		},
		{
			"No matching metric name w/ rolled-up dims",
			"c",
			map[string]string{
				"dim1": "foo",
			},
			[][]string{{"dim1"}},
			[][]string{{"dim1"}},
		},
	}

	for _, tc := range testCases {
		metric := pdata.NewMetric()
		metric.InitEmpty()
		metric.SetName(tc.metricName)
		t.Run(tc.testName, func(t *testing.T) {
			dimensions := processMetricDeclarations(metricDeclarations, &metric, tc.labels, tc.rollUpDims)
			assert.Equal(t, tc.expectedDims, dimensions)
		})
	}
}
