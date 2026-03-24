// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestUnstructuredToEndpointValidationErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil_object", func(t *testing.T) {
		t.Parallel()
		_, err := unstructuredToEndpoint("ns", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("nil_object_root", func(t *testing.T) {
		t.Parallel()
		_, err := unstructuredToEndpoint("ns", &unstructured.Unstructured{Object: nil})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "root")
	})

	t.Run("missing_uid", func(t *testing.T) {
		t.Parallel()
		u := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "x",
				},
			},
		}
		_, err := unstructuredToEndpoint("ns", u)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uid")
	})

	t.Run("missing_name", func(t *testing.T) {
		t.Parallel()
		u := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"uid": "abc",
				},
			},
		}
		u.SetUID(types.UID("abc"))
		_, err := unstructuredToEndpoint("ns", u)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})
}
