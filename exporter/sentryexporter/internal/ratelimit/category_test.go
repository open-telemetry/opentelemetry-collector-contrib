// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"testing"
)

func TestCategory_String(t *testing.T) {
	tests := []struct {
		category Category
		expected string
	}{
		{CategoryAll, "CategoryAll"},
		{CategoryTransaction, "CategoryTransaction"},
		{CategoryLog, "CategoryLog"},
		{Category("custom type"), "CategoryCustomType"},
		{Category("multi word type"), "CategoryMultiWordType"},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			result := tt.category.String()
			if result != tt.expected {
				t.Errorf("Category(%q).String() = %q, want %q", tt.category, result, tt.expected)
			}
		})
	}
}

func TestKnownCategories(t *testing.T) {
	expectedCategories := []Category{
		CategoryAll,
		CategoryTransaction,
		CategoryLog,
	}

	for _, category := range expectedCategories {
		t.Run(string(category), func(t *testing.T) {
			if _, exists := knownCategories[category]; !exists {
				t.Errorf("Category %q should be in knownCategories map", category)
			}
		})
	}

	// Test that unknown categories are not in the map
	unknownCategories := []Category{
		Category("unknown"),
		Category("custom"),
		Category("random"),
	}

	for _, category := range unknownCategories {
		t.Run("unknown_"+string(category), func(t *testing.T) {
			if _, exists := knownCategories[category]; exists {
				t.Errorf("Unknown category %q should not be in knownCategories map", category)
			}
		})
	}
}
