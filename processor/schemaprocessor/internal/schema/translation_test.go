package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslation_SupportedVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario  string
		schema    Schema
		version   *Version
		supported bool
	}{
		{
			scenario:  "Known supported version",
			schema:    newTranslation(&Version{1, 2, 1}),
			version:   &Version{1, 0, 0},
			supported: true,
		},
		{
			scenario:  "Unsupported version",
			schema:    newTranslation(&Version{1, 0, 0}),
			version:   &Version{1, 33, 7},
			supported: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			assert.Equal(
				t,
				tc.supported,
				tc.schema.SupportedVersion(tc.version),
				"Must match the expected supported version",
			)
		})
	}
}
