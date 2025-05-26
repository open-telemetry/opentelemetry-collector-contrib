// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logprofile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
)

func TestProfile_MarshalLogObject(t *testing.T) {
	tests := []struct {
		name        string
		profile     pprofile.Profile
		contains    []string
		notContains []string
	}{
		{
			name: "valid",
			profile: func() pprofile.Profile {
				p := &pprofiletest.Profile{
					ProfileID:  pprofile.ProfileID([]byte("profileid1111111")),
					Attributes: []pprofiletest.Attribute{{Key: "container-attr1", Value: "value1"}},
				}
				return p.Transform(pprofile.NewScopeProfiles())
			}(),
			notContains: []string{"profileError"},
		},
		{
			name:     "invalid",
			profile:  pprofile.NewProfile(), // doesn't include the required empty string in stringTable
			contains: []string{"profileError", "string index out of bounds: 0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{})
			buf, err := encoder.EncodeEntry(zapcore.Entry{}, []zapcore.Field{zap.Object("profile", Profile(tt.profile))})
			assert.NoError(t, err)

			for _, s := range tt.contains {
				assert.Contains(t, buf.String(), s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, buf.String(), s)
			}
		})
	}
}
