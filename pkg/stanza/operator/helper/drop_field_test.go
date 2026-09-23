// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestDropFieldConfigValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		dropField   bool
		parseFrom   entry.Field
		expectedErr string
	}{
		{
			name:      "disabled_root_body",
			dropField: false,
			parseFrom: entry.NewBodyField(),
		},
		{
			name:        "root_body",
			dropField:   true,
			parseFrom:   entry.NewBodyField(),
			expectedErr: "`drop_field: true` cannot be used when `parse_from` is `body`",
		},
		{
			name:        "root_attributes",
			dropField:   true,
			parseFrom:   entry.NewAttributeField(),
			expectedErr: "`drop_field: true` cannot be used when `parse_from` is `attributes`",
		},
		{
			name:        "root_resource",
			dropField:   true,
			parseFrom:   entry.NewResourceField(),
			expectedErr: "`drop_field: true` cannot be used when `parse_from` is `resource`",
		},
		{
			name:      "valid_body_subfield",
			dropField: true,
			parseFrom: entry.NewBodyField("message"),
		},
		{
			name:      "valid_attribute_subfield",
			dropField: true,
			parseFrom: entry.NewAttributeField("raw"),
		},
		{
			name:      "valid_resource_subfield",
			dropField: true,
			parseFrom: entry.NewResourceField("raw"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DropFieldConfig{DropField: tc.dropField}
			err := cfg.Validate(tc.parseFrom)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDropFieldConfigValidateWithTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		dropField   bool
		parseFrom   entry.Field
		parseTo     entry.Field
		expectedErr string
	}{
		{
			name:      "disabled_same_field",
			dropField: false,
			parseFrom: entry.NewBodyField("message"),
			parseTo:   entry.NewBodyField("message"),
		},
		{
			name:        "root_body",
			dropField:   true,
			parseFrom:   entry.NewBodyField(),
			parseTo:     entry.NewAttributeField(),
			expectedErr: "`drop_field: true` cannot be used when `parse_from` is `body`",
		},
		{
			name:        "same_field",
			dropField:   true,
			parseFrom:   entry.NewBodyField("message"),
			parseTo:     entry.NewBodyField("message"),
			expectedErr: "`parse_to` and `parse_from` cannot be the same when `drop_field: true`",
		},
		{
			name:        "parse_to_subfield_of_parse_from",
			dropField:   true,
			parseFrom:   entry.NewAttributeField("raw"),
			parseTo:     entry.NewAttributeField("raw", "parsed"),
			expectedErr: "`parse_to` cannot be a subfield of `parse_from` when `drop_field: true`",
		},
		{
			name:      "parse_from_subfield_of_parse_to_root",
			dropField: true,
			parseFrom: entry.NewAttributeField("raw"),
			parseTo:   entry.NewAttributeField(),
		},
		{
			name:      "sibling_subfields",
			dropField: true,
			parseFrom: entry.NewBodyField("raw"),
			parseTo:   entry.NewBodyField("parsed"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DropFieldConfig{DropField: tc.dropField}
			err := cfg.ValidateWithTarget(tc.parseFrom, tc.parseTo)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
