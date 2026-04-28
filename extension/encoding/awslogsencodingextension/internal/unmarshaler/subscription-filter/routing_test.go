// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func mustNewID(t *testing.T, s string) component.ID {
	t.Helper()
	var id component.ID
	require.NoError(t, id.UnmarshalText([]byte(s)))
	return id
}

func TestValidateRoutes(t *testing.T) {
	t.Parallel()

	encA := mustNewID(t, "fake/a")
	encB := mustNewID(t, "fake/b")

	tests := []struct {
		name    string
		routes  []CloudWatchRoute
		wantErr string // substring; empty means no error
	}{
		{
			name: "valid: name with explicit log group",
			routes: []CloudWatchRoute{
				{Name: "lambda-payments", LogGroupPattern: "/aws/lambda/payment-*", Encoding: encA},
			},
		},
		{
			name: "valid: name with explicit log stream",
			routes: []CloudWatchRoute{
				{Name: "vpc-eni", LogStreamPattern: "eni-*", Encoding: encA},
			},
		},
		{
			name: "valid: known name with no patterns (defaults will apply)",
			routes: []CloudWatchRoute{
				{Name: "vpcflow", Encoding: encA},
			},
		},
		{
			name: "valid: known name with explicit pattern (overrides default)",
			routes: []CloudWatchRoute{
				{Name: "lambda", LogGroupPattern: "/custom/*", Encoding: encA},
			},
		},
		{
			name: "missing name",
			routes: []CloudWatchRoute{
				{LogGroupPattern: "/aws/lambda/*", Encoding: encA},
			},
			wantErr: "'name' is required",
		},
		{
			name: "unknown name with no patterns",
			routes: []CloudWatchRoute{
				{Name: "unknown-service", Encoding: encA},
			},
			wantErr: `name "unknown-service" has no defaults`,
		},
		{
			name: "missing encoding",
			routes: []CloudWatchRoute{
				{Name: "foo", LogGroupPattern: "/aws/lambda/*"},
			},
			wantErr: "'encoding' is required",
		},
		{
			name: "duplicate name",
			routes: []CloudWatchRoute{
				{Name: "vpcflow", Encoding: encA},
				{Name: "vpcflow", Encoding: encB},
			},
			wantErr: `duplicate name "vpcflow"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateRoutes(tt.routes)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestWithDefaults(t *testing.T) {
	t.Parallel()

	encID := mustNewID(t, "fake/a")

	tests := []struct {
		name string
		in   CloudWatchRoute
		want CloudWatchRoute
	}{
		{
			name: "known name no patterns -> default applied",
			in:   CloudWatchRoute{Name: "vpcflow", Encoding: encID},
			want: CloudWatchRoute{Name: "vpcflow", Encoding: encID, LogStreamPattern: "eni-*"},
		},
		{
			name: "known name with explicit pattern -> default skipped",
			in:   CloudWatchRoute{Name: "vpcflow", LogGroupPattern: "/custom/*", Encoding: encID},
			want: CloudWatchRoute{Name: "vpcflow", LogGroupPattern: "/custom/*", Encoding: encID},
		},
		{
			name: "unknown name -> route returned unchanged",
			in:   CloudWatchRoute{Name: "weird", Encoding: encID},
			want: CloudWatchRoute{Name: "weird", Encoding: encID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.in.withDefaults())
		})
	}
}

func TestSortRoutes(t *testing.T) {
	t.Parallel()

	encID := mustNewID(t, "fake/a")
	mk := func(group, stream string) CloudWatchRoute {
		return CloudWatchRoute{LogGroupPattern: group, LogStreamPattern: stream, Encoding: encID}
	}

	tests := []struct {
		name  string
		input []CloudWatchRoute
		want  []CloudWatchRoute
	}{
		{
			name: "catch-all goes last",
			input: []CloudWatchRoute{
				mk("*", ""),
				mk("/aws/lambda/*", ""),
			},
			want: []CloudWatchRoute{
				mk("/aws/lambda/*", ""),
				mk("*", ""),
			},
		},
		{
			name: "log_group before log_stream-only",
			input: []CloudWatchRoute{
				mk("", "eni-*"),
				mk("/aws/lambda/*", ""),
			},
			want: []CloudWatchRoute{
				mk("/aws/lambda/*", ""),
				mk("", "eni-*"),
			},
		},
		{
			name: "more specific group pattern first",
			input: []CloudWatchRoute{
				mk("/aws/lambda/*", ""),
				mk("/aws/lambda/payment-*", ""),
			},
			want: []CloudWatchRoute{
				mk("/aws/lambda/payment-*", ""),
				mk("/aws/lambda/*", ""),
			},
		},
		{
			name: "three-level interaction",
			input: []CloudWatchRoute{
				mk("*", ""),                     // catch-all
				mk("", "eni-*"),                 // stream-only
				mk("/aws/lambda/*", ""),         // generic group
				mk("/aws/lambda/payment-*", ""), // specific group
			},
			want: []CloudWatchRoute{
				mk("/aws/lambda/payment-*", ""),
				mk("/aws/lambda/*", ""),
				mk("", "eni-*"),
				mk("*", ""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SortRoutes(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSortRoutesAppliesDefaults(t *testing.T) {
	t.Parallel()

	encID := mustNewID(t, "fake/a")
	in := []CloudWatchRoute{
		{Name: "lambda", Encoding: encID},  // expands to LogGroupPattern: /aws/lambda/*
		{Name: "vpcflow", Encoding: encID}, // expands to LogStreamPattern: eni-*
	}
	got := SortRoutes(in)

	// After defaults: lambda has a log_group_pattern, vpcflow has only log_stream_pattern.
	// Sort places log_group entries before log_stream-only entries.
	require.Len(t, got, 2)
	assert.Equal(t, "/aws/lambda/*", got[0].LogGroupPattern)
	assert.Equal(t, "eni-*", got[1].LogStreamPattern)
}

func TestSortRoutesEmpty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, SortRoutes(nil))
	assert.Nil(t, SortRoutes([]CloudWatchRoute{}))
}
