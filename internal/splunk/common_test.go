// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetValues(t *testing.T) {
	metric := Event{
		Fields: map[string]interface{}{},
	}
	assert.Equal(t, map[string]interface{}{}, metric.GetMetricValues())
	metric.Fields["metric_name:foo"] = "bar"
	assert.Equal(t, map[string]interface{}{"foo": "bar"}, metric.GetMetricValues())
	metric.Fields["metric_name:foo2"] = "foobar"
	assert.Equal(t, map[string]interface{}{"foo": "bar", "foo2": "foobar"}, metric.GetMetricValues())
}

func TestIsMetric(t *testing.T) {
	ev := Event{
		Event: map[string]interface{}{},
	}
	assert.False(t, ev.IsMetric())
	metric := Event{
		Event: "metric",
	}
	assert.True(t, metric.IsMetric())
	arr := Event{
		Event: []interface{}{"foo", "bar"},
	}
	assert.False(t, arr.IsMetric())
	yo := Event{
		Event: "yo",
	}
	assert.False(t, yo.IsMetric())
}

func TestIsMetric_WithoutEventField(t *testing.T) {
	fieldsOnly := Event{
		Fields: map[string]interface{}{
			"foo": "bar",
		},
	}
	assert.False(t, fieldsOnly.IsMetric())
	fieldsWithMetrics := Event{
		Fields: map[string]interface{}{
			"foo":             "bar",
			"metric_name:foo": 123,
			"foobar":          "foobar",
		},
	}
	assert.True(t, fieldsWithMetrics.IsMetric())
}

func TestDecodeJsonWithNoTime(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader("{\"event\":\"hello\"}"))

	dec.More()
	var msg Event
	err := dec.Decode(&msg)
	assert.NoError(t, err)
	assert.Zero(t, msg.Time)
}

func TestDecodeJsonWithNumberTime(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader("{\"time\":1610760752.606,\"event\":\"hello\"}"))

	dec.More()
	var msg Event
	err := dec.Decode(&msg)
	assert.NoError(t, err)
	assert.Equal(t, 1610760752.606, msg.Time)
}

func TestDecodeJsonWithStringTime(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader("{\"time\":\"1610760752.606\",\"event\":\"hello\"}"))

	dec.More()
	var msg Event
	err := dec.Decode(&msg)
	assert.NoError(t, err)
	assert.Equal(t, 1610760752.606, msg.Time)
}

func TestDecodeJsonWithInvalidStringTime(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader("{\"time\":\"1610760752.606\\\"\",\"event\":\"hello\"}"))

	dec.More()
	var msg Event
	err := dec.Decode(&msg)
	assert.Error(t, err)
}

func TestDecodeJsonWithInvalidNumberStringTime(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader("{\"time\":\"0xdeadbeef\",\"event\":\"hello\"}"))

	dec.More()
	var msg Event
	err := dec.Decode(&msg)
	assert.Error(t, err)
}

func TestDecodeJsonWithInvalidNumberTime(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader("{\"time\":1e1024,\"event\":\"hello\"}"))

	dec.More()
	var msg Event
	err := dec.Decode(&msg)
	assert.Error(t, err)
}
