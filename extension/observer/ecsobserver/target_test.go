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
