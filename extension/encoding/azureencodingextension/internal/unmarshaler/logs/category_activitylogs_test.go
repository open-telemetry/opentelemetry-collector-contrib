// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPimCallerKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		// Known CallerIdentityType values from real PIM events
		{input: "UPN", want: "upn"},
		{input: "ObjectID", want: "object_id"},
		{input: "Username", want: "username"},
		{input: "Name", want: "name"},
		{input: "ObjectClass", want: "object_class"},
		// Other plausible Microsoft identity types
		{input: "ID", want: "id"},
		{input: "DisplayName", want: "display_name"},
		{input: "SomeHTTPField", want: "some_http_field"},
		{input: "ServicePrincipalID", want: "service_principal_id"},
		// Edge cases
		{input: "", want: ""},
		{input: "A", want: "a"},
		{input: "alreadylower", want: "alreadylower"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, pimCallerKey(tt.input))
		})
	}
}
