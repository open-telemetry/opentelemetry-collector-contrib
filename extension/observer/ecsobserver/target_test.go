// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTargetToLabels(t *testing.T) {
	t.Run("sanitize tags", func(t *testing.T) {
		pt := prometheusECSTarget{
			TaskTags: map[string]string{
				"a:b": "sanitized",
				"ab":  "same",
			},
		}
		m := pt.ToLabels()
		assert.Equal(t, "sanitized", m["__meta_ecs_task_tags_a_b"])
		assert.Equal(t, "same", m["__meta_ecs_task_tags_ab"])
	})
}

func TestTargetsToFileSDTargets(t *testing.T) {
	targets := []prometheusECSTarget{
		{
			Address: "192.168.1.1:9090",
			Job:     "test-job",
		},
	}

	t.Run("job label renamed when custom job_label_name provided", func(t *testing.T) {
		result, err := targetsToFileSDTargets(targets, "prometheus_job")
		assert.NoError(t, err)
		assert.Len(t, result, 1)

		labels := result[0].Labels
		assert.Equal(t, "test-job", labels["prometheus_job"])
		assert.NotContains(t, labels, "job")
	})

	t.Run("job label kept when job_label_name is 'job'", func(t *testing.T) {
		result, err := targetsToFileSDTargets(targets, "job")
		assert.NoError(t, err)
		assert.Len(t, result, 1)

		labels := result[0].Labels
		assert.Equal(t, "test-job", labels["job"])
		assert.NotContains(t, labels, "prometheus_job")
	})

	t.Run("job label kept when job_label_name is empty string", func(t *testing.T) {
		result, err := targetsToFileSDTargets(targets, "")
		assert.NoError(t, err)
		assert.Len(t, result, 1)

		labels := result[0].Labels
		assert.Equal(t, "test-job", labels["job"])
		assert.NotContains(t, labels, "")
	})

	t.Run("no job relabeling when job is empty", func(t *testing.T) {
		emptyJobTargets := []prometheusECSTarget{
			{
				Address: "192.168.1.1:9090",
				Job:     "",
			},
		}
		result, err := targetsToFileSDTargets(emptyJobTargets, "prometheus_job")
		assert.NoError(t, err)
		assert.Len(t, result, 1)

		labels := result[0].Labels
		assert.NotContains(t, labels, "job")
		assert.NotContains(t, labels, "prometheus_job")
	})
}
