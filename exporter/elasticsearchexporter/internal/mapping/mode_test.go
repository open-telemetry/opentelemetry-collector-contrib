// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mapping

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFind(t *testing.T) {
	for _, tt := range []struct {
		name string

		mode     string
		wantMode Mode
	}{
		{
			name:     "with an existing mode",
			mode:     "ecs",
			wantMode: ModeECS,
		},
		{
			name:     "with an uknown mode",
			mode:     "nothing",
			wantMode: ModeInvalid,
		},
		{
			name:     "with the no alias",
			mode:     "no",
			wantMode: ModeNone,
		},
		{
			name:     "with the none alias",
			mode:     "none",
			wantMode: ModeNone,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantMode, Find(tt.mode))
		})
	}
}
