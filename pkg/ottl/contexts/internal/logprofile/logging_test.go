// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logprofile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
)

func TestProfile_MarshalLogObject(t *testing.T) {
	tests := []struct {
		name        string
		profile     func() (pprofile.ProfilesDictionary, pprofile.Profile)
		contains    []string
		notContains []string
	}{
		{
			name: "valid",
			profile: func() (pprofile.ProfilesDictionary, pprofile.Profile) {
				dic := pprofile.NewProfilesDictionary()
				p := &pprofiletest.Profile{
					ProfileID: pprofile.ProfileID([]byte("profileid1111111")),
					SampleType: []pprofiletest.ValueType{
						{
							Typ:  "test",
							Unit: "foobar",
						},
					},
					Sample: []pprofiletest.Sample{
						{
							Locations: []pprofiletest.Location{
								{
									Address: 0x42,
								},
							},
							Values: []int64{73},
							Link: &pprofiletest.Link{
								TraceID: pcommon.TraceID{
									0xf, 0xe, 0xd, 0xc, 0xb, 0xa, 0x9, 0x8,
									0x7, 0x6, 0x5, 0x4, 0x3, 0x2, 0x1, 0x0,
								},
								SpanID: pcommon.SpanID{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7},
							},
						},
						{
							Locations: []pprofiletest.Location{
								{
									Address: 0x43,
								},
							},
							Values: []int64{74},
							Attributes: []pprofiletest.Attribute{
								{Key: "sample2", Value: "value2"},
							},
						},
					},
					Attributes: []pprofiletest.Attribute{{Key: "container-attr1", Value: "value1"}},
				}
				return dic, p.Transform(dic, pprofile.NewScopeProfiles())
			},
			notContains: []string{"profileError"},
		},
		{
			name: "invalid",
			profile: func() (pprofile.ProfilesDictionary, pprofile.Profile) {
				return pprofile.NewProfilesDictionary(), pprofile.NewProfile()
			},
			contains: []string{"profileError", "string index out of bounds: 0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dic, prof := tt.profile()
			encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{})
			buf, err := encoder.EncodeEntry(zapcore.Entry{}, []zapcore.Field{zap.Object("profile", Profile{prof, dic})})
			require.NoError(t, err)

			for _, s := range tt.contains {
				assert.Contains(t, buf.String(), s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, buf.String(), s)
			}
		})
	}
}
