//go:build cgo
// +build cgo

package timeutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseStrptimeCgo(t *testing.T) {
	// Verify that libc's strptime parses these in the same way.

	for _, tt := range strptimeTests {
		t.Run(tt.name, func(t *testing.T) {
			for _, s := range tt.samples {
				t.Run(s, func(t *testing.T) {
					got, err := C_strptime(s, tt.format)
					require.NoError(t, err)
					// Use WithinDuration instead of Equal so the timezone name is ignored.
					require.WithinDuration(t, tt.expected, got, 0)
				})
			}
		})
	}
}
