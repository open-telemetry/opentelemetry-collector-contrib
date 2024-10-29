// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateDefaultProcessorConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.Equal(t, defaultInterval, cfg.Interval)
	require.Equal(t, defaultLogCountAttribute, cfg.LogCountAttribute)
	require.Equal(t, defaultTimezone, cfg.Timezone)
	require.Equal(t, []string{}, cfg.ExcludeFields)
}

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "invalid LogCountAttribute config",
			cfg: &Config{
				LogCountAttribute: "",
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
			},
			expectedErr: errInvalidLogCountAttribute,
		},
		{
			desc: "invalid Interval config",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          -1,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
			},
			expectedErr: errInvalidInterval,
		},
		{
			desc: "invalid Timezone config",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          "not a timezone",
				ExcludeFields:     []string{},
			},
			expectedErr: errors.New("timezone is invalid"),
		},
		{
			desc: "invalid exclude entire body",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{bodyField},
			},
			expectedErr: errCannotExcludeBody,
		},
		{
			desc: "invalid exclude field body",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{"not.value"},
			},
			expectedErr: errors.New("an excludefield must start with"),
		},
		{
			desc: "invalid duplice exclude field",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{"body.thing", "body.thing"},
			},
			expectedErr: errors.New("duplicate exclude_field"),
		},
		{
			desc: "valid config",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				Conditions:        []string{},
				ExcludeFields:     []string{"body.thing", "attributes.otherthing"},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
